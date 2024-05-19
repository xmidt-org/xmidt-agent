// SPDX-FileCopyrightText: 2023 Anmol Sethi <hi@nhooyr.io>
// SPDX-License-Identifier: ISC

//go:build !js
// +build !js

package websocket

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// MessageType represents the type of a WebSocket message.
// See https://tools.ietf.org/html/rfc6455#section-5.6
type MessageType int

// MessageType constants.
const (
	// MessageText is for UTF-8 encoded text messages like JSON.
	MessageText MessageType = iota + 1
	// MessageBinary is for binary messages like protobufs.
	MessageBinary
)

// Conn represents a WebSocket connection.
// All methods may be called concurrently except for Reader and Read.
//
// You must always read from the connection. Otherwise control
// frames will not be handled. See Reader and CloseRead.
//
// Be sure to call Close on the connection when you
// are finished with it to release associated resources.
//
// On any error from any method, the connection is closed
// with an appropriate reason.
//
// This applies to context expirations as well unfortunately.
// See https://github.com/nhooyr/websocket/issues/242#issuecomment-633182220
//
// Connection closures due to inactivity timeouts (no reads & writes) will close
// with the StatusCode `StatusInactivityTimeout = 4000`and the opcode `opClose = 8“.
type Conn struct {
	noCopy noCopy

	subprotocol    string
	rwc            io.ReadWriteCloser
	client         bool
	copts          *compressionOptions
	flateThreshold int
	br             *bufio.Reader
	bw             *bufio.Writer

	readTimeout  chan context.Context
	writeTimeout chan context.Context

	// Read state.
	readMu            *mu
	readHeaderBuf     [8]byte
	readControlBuf    [maxControlPayload]byte
	msgReader         *msgReader
	readCloseFrameErr error

	// Write state.
	msgWriter      *msgWriter
	writeFrameMu   *mu
	writeBuf       []byte
	writeHeaderBuf [8]byte
	writeHeader    header

	wg         sync.WaitGroup
	closed     chan struct{}
	closeMu    sync.Mutex
	closeErr   error
	wroteClose bool

	inactivityTimeout time.Duration
	pingWriteTimeout  time.Duration
	pingCounter       int32
	activePingsMu     sync.Mutex
	activePings       map[string]chan<- struct{}
	pingListener      func(context.Context, []byte)
	pongListener      func(context.Context, []byte)
}

type connConfig struct {
	subprotocol    string
	rwc            io.ReadWriteCloser
	client         bool
	copts          *compressionOptions
	flateThreshold int

	br *bufio.Reader
	bw *bufio.Writer

	inactivityTimeout time.Duration
}

func newConn(cfg connConfig, opts ...ConnOption) *Conn {
	c := &Conn{
		subprotocol:    cfg.subprotocol,
		rwc:            cfg.rwc,
		client:         cfg.client,
		copts:          cfg.copts,
		flateThreshold: cfg.flateThreshold,

		br: cfg.br,
		bw: cfg.bw,

		inactivityTimeout: cfg.inactivityTimeout,

		readTimeout:  make(chan context.Context),
		writeTimeout: make(chan context.Context),

		closed:      make(chan struct{}),
		activePings: make(map[string]chan<- struct{}),
	}
	// set default ping, pong handler
	c.SetPingListener(nil)
	c.SetPongListener(nil)

	c.readMu = newMu(c)
	c.writeFrameMu = newMu(c)

	c.msgReader = newMsgReader(c)

	c.msgWriter = newMsgWriter(c)
	if c.client {
		c.writeBuf = extractBufioWriterBuf(c.bw, c.rwc)
	}

	if c.flate() && c.flateThreshold == 0 {
		c.flateThreshold = 128
		if !c.msgWriter.flateContextTakeover() {
			c.flateThreshold = 512
		}
	}

	runtime.SetFinalizer(c, func(c *Conn) {
		c.close(errors.New("connection garbage collected"))
	})

	for _, opt := range opts {
		if opt != nil {
			opt.apply(c)
		}
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.timeoutLoop()
	}()

	return c
}

// Subprotocol returns the negotiated subprotocol.
// An empty string means the default protocol.
func (c *Conn) Subprotocol() string {
	return c.subprotocol
}

func (c *Conn) close(err error) {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()

	if c.isClosed() {
		return
	}
	if err == nil {
		err = c.rwc.Close()
	}
	c.setCloseErrLocked(err)

	close(c.closed)
	runtime.SetFinalizer(c, nil)

	// Have to close after c.closed is closed to ensure any goroutine that wakes up
	// from the connection being closed also sees that c.closed is closed and returns
	// closeErr.
	c.rwc.Close()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.msgWriter.close()
		c.msgReader.close()
	}()
}

func (c *Conn) timeoutLoop() {
	// `activityProbe`` is used to monitor reads and writes activity until the connection is closed.
	activityProbe := c.monitorConnectionActivity()
	defer close(activityProbe)

	readCtx := context.Background()
	writeCtx := context.Background()

	for {
		select {
		case <-c.closed:
			return

		case writeCtx = <-c.writeTimeout:
			// Note, all writes will enqueue their context in `c.writeTimeout`
			activityProbe <- struct{}{}
		case readCtx = <-c.readTimeout:
			// Note, all reads will enqueue their context in `c.readTimeout`
			activityProbe <- struct{}{}

		case <-readCtx.Done():
			c.setCloseErr(fmt.Errorf("read timed out: %w", readCtx.Err()))
			c.wg.Add(1)
			go func() {
				defer c.wg.Done()
				c.writeError(StatusPolicyViolation, errors.New("read timed out"))
			}()
		case <-writeCtx.Done():
			c.close(fmt.Errorf("write timed out: %w", writeCtx.Err()))
			return
		}
	}
}

// monitorConnectionActivity determines whether the connection is active by monitoring for reads & writes activity. If the
// connection is inactivity and `inactivityTimeout` is triggered, then the connection will be
// closed with the StatusCode `StatusInactivityTimeout  = 4000`and the opcode `opClose = 8“.
func (c *Conn) monitorConnectionActivity() (activityProbe chan struct{}) {
	// `activityProbe` probes for connection activity.
	activityProbe = make(chan struct{})
	// done signals to stop monitoring for activity since this connection is actively closing.
	done := make(chan struct{})

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		// Actively monitor `activityProbe` for any connection activity (reads & writes).
		for {
			inactivityTimeout := time.After(1 * time.Minute)
			if c.inactivityTimeout > 0 {
				inactivityTimeout = time.After(c.inactivityTimeout)
			}

			select {
			// When `activityProbe` is closed, then the connection has successfully closed
			// and no additional activity is expected, close `activity`.
			case _, ok := <-activityProbe:
				// Connection activity has been observed, reset the `inactivityTimeout` timer.
				// Continue ingesting activity until `c.closed` is closed.
				if !ok {
					// Connection has fully closed, stop monitoring for activity.
					return
				}
			case <-done:
				// After the first `inactivityTimeout` is triggered, `inactivityTimeout`
				// is no longer listened to since this connection is actively closing.
			case <-inactivityTimeout:
				// No connection activity was observed, triggering `inactivityTimeout`.
				// Start closing this connection.
				close(done)
				// We do this after in case there was an error writing the close frame.
				c.setCloseErr(errors.New("inactivity timed out"))
				c.wg.Add(1)
				// Close connection.
				go func() {
					defer c.wg.Done()
					c.writeError(StatusInactivityTimeout, errors.New("inactivity timed out"))
				}()
			}
		}
	}()

	return activityProbe
}

func (c *Conn) flate() bool {
	return c.copts != nil
}

// Ping sends a ping to the peer and waits for a pong.
// Use this to measure latency or ensure the peer is responsive.
// Ping must be called concurrently with Reader as it does
// not read from the connection but instead waits for a Reader call
// to read the pong.
//
// TCP Keepalives should suffice for most use cases.
func (c *Conn) Ping(ctx context.Context) error {
	p := atomic.AddInt32(&c.pingCounter, 1)

	err := c.ping(ctx, strconv.Itoa(int(p)))
	if err != nil {
		return fmt.Errorf("failed to ping: %w", err)
	}
	return nil
}

func (c *Conn) ping(ctx context.Context, p string) error {
	pong := make(chan struct{}, 1)

	c.activePingsMu.Lock()
	c.activePings[p] = pong
	c.activePingsMu.Unlock()

	defer func() {
		c.activePingsMu.Lock()
		delete(c.activePings, p)
		c.activePingsMu.Unlock()
	}()

	err := c.writeControl(ctx, opPing, []byte(p))
	if err != nil {
		return err
	}

	select {
	case <-c.closed:
		return net.ErrClosed
	case <-ctx.Done():
		err := fmt.Errorf("failed to wait for pong: %w", ctx.Err())
		c.close(err)
		return err
	case <-pong:
		return nil
	}
}

type mu struct {
	c  *Conn
	ch chan struct{}
}

func newMu(c *Conn) *mu {
	return &mu{
		c:  c,
		ch: make(chan struct{}, 1),
	}
}

func (m *mu) forceLock() {
	m.ch <- struct{}{}
}

func (m *mu) tryLock() bool {
	select {
	case m.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

func (m *mu) lock(ctx context.Context) error {
	select {
	case <-m.c.closed:
		return net.ErrClosed
	case <-ctx.Done():
		err := fmt.Errorf("failed to acquire lock: %w", ctx.Err())
		m.c.close(err)
		return err
	case m.ch <- struct{}{}:
		// To make sure the connection is certainly alive.
		// As it's possible the send on m.ch was selected
		// over the receive on closed.
		select {
		case <-m.c.closed:
			// Make sure to release.
			m.unlock()
			return net.ErrClosed
		default:
		}
		return nil
	}
}

func (m *mu) unlock() {
	select {
	case <-m.ch:
	default:
	}
}

type noCopy struct{}

func (*noCopy) Lock() {}
