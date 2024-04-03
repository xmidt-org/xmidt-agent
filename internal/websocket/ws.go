// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/xmidt-org/eventor"
	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/websocket/event"
	nhws "nhooyr.io/websocket"
)

var (
	ErrMisconfiguredWS = errors.New("misconfigured WS")
	ErrClosed          = errors.New("websocket closed")
	ErrInvalidMsgType  = errors.New("invalid message type")
)

// emptyBuffer is solely used as an address of a global empty buffer.
// This sentinel value will reset pointers of the writePump's encoder
// such that the gc can clean things up.
var emptyBuffer = []byte{}

type Websocket struct {
	// id is the device ID for the WS connection.
	id wrp.DeviceID

	// urlFetcher is the URLFetcher for the WS connection.
	urlFetcher func(context.Context) (string, error)

	// urlFetchingTimeout is the URLFetchingTimeout for the WS connection.
	urlFetchingTimeout time.Duration

	// credDecorator is the credentials decorator for the WS connection.
	credDecorator func(http.Header) error

	// pingInterval is the ping interval allowed for the WS connection.
	pingInterval time.Duration

	// pingTimeout is the ping timeout for the WS connection.
	pingTimeout time.Duration

	// connectTimeout is the connect timeout for the WS connection.
	connectTimeout time.Duration

	// keepAliveInterval is the keep alive interval for the WS connection.
	keepAliveInterval time.Duration

	// idleConnTimeout is the idle connection timeout for the WS connection.
	idleConnTimeout time.Duration

	// tlsHandshakeTimeout is the TLS handshake timeout for the WS connection.
	tlsHandshakeTimeout time.Duration

	// expectContinueTimeout is the expect continue timeout for the WS connection.
	expectContinueTimeout time.Duration

	// additionalHeaders are any additional headers for the WS connection.
	additionalHeaders http.Header

	// maxMessageBytes is the largest allowable message to send or receive.
	maxMessageBytes int64

	// withIPv4 is whether or not to allow IPv4 for the WS connection.
	withIPv4 bool

	// withIPv6 is whether or not to allow IPv6 for the WS connection.
	withIPv6 bool

	// connectListeners are the connect listeners for the WS connection.
	connectListeners eventor.Eventor[event.ConnectListener]

	// disconnectListeners are the disconnect listeners for the WS connection.
	disconnectListeners eventor.Eventor[event.DisconnectListener]

	// heartbeatListeners are the heartbeat listeners for the WS connection.
	heartbeatListeners eventor.Eventor[event.HeartbeatListener]

	// msgListeners are the message listeners for messages from the WS.
	msgListeners eventor.Eventor[event.MsgListener]

	// nowFunc is the now function for the WS connection.
	nowFunc func() time.Time

	// retryPolicyFactory is the retry policy factory for the WS connection.
	retryPolicyFactory retry.PolicyFactory

	// once is whether or not to only attempt to connect once.
	once bool

	m        sync.Mutex
	wg       sync.WaitGroup
	shutdown context.CancelFunc

	mode    ipMode
	policy  retry.Policy
	decoder wrp.Decoder
	encoder wrp.Encoder
	conn    *nhws.Conn
}

// Option is a functional option type for WS.
type Option interface {
	apply(*Websocket) error
}

type optionFunc func(*Websocket) error

func (f optionFunc) apply(c *Websocket) error {
	return f(c)
}

// New creates a new WS connection with the given options.
func New(opts ...Option) (*Websocket, error) {
	var ws Websocket

	opts = append(opts,
		validateDeviceID(),
		validateURL(),
		validateIPMode(),
		validateFetchURL(),
		validateCredentialsDecorator(),
		validateNowFunc(),
		validRetryPolicy(),
	)

	for _, opt := range opts {
		if opt != nil {
			if err := opt.apply(&ws); err != nil {
				return nil, err
			}
		}
	}

	return &ws, nil
}

// Start starts the websocket connection and a long running goroutine to maintain
// the connection.
func (ws *Websocket) Start() {
	ws.m.Lock()
	defer ws.m.Unlock()

	if ws.shutdown != nil {
		return
	}

	var ctx context.Context
	ctx, ws.shutdown = context.WithCancel(context.Background())
	ws.mode = ipv4
	// Init retry policy, but it'll be reset on recurring successful connections.
	ws.policy = ws.retryPolicyFactory.NewPolicy(ctx)
	ws.decoder = wrp.NewDecoder(nil, wrp.Msgpack)

	go ws.read(ctx)
}

// Stop stops the websocket connection.
func (ws *Websocket) Stop() {
	ws.m.Lock()
	// Avoid the overhead of the close handshake.
	ws.conn.Close(nhws.StatusNormalClosure, "")
	shutdown := ws.shutdown
	ws.m.Unlock()

	if shutdown != nil {
		shutdown()
	}

	ws.wg.Wait()
}

// Send sends the provided WRP message through the existing websocket.  This
// call synchronously blocks until the write is complete.
func (ws *Websocket) Send(ctx context.Context, msg wrp.Message) error {
	err := ErrClosed

	ws.m.Lock()
	if ws.conn != nil {
		err = ws.conn.Write(ctx, nhws.MessageBinary, wrp.MustEncode(&msg, wrp.Msgpack))
	}
	ws.m.Unlock()

	return err
}

func (ws *Websocket) read(ctx context.Context) {
	ws.wg.Add(1)
	defer ws.wg.Done()

	reconnect := true
	for {
		var dialErr error
		if reconnect {
			dialErr = ws.dial(ctx)
		}

		if dialErr == nil {
			// Read loop
			for {
				msg, err := ws.readMsg(ctx)
				// If a reconnect was attempted but failed, ErrClosed will be found
				// in this error list and a reconnect should be attempted again.
				reconnect = errors.Is(err, ErrClosed)
				if err != nil {
					break
				}

				ws.msgListeners.Visit(func(l event.MsgListener) {
					l.OnMessage(*msg)
				})
			}
		}

		if ws.once {
			return
		}

		next, _ := ws.policy.Next()

		select {
		case <-time.After(next):
		case <-ctx.Done():
			return
		}
	}
}

func (ws *Websocket) readMsg(ctx context.Context) (msg *wrp.Message, err error) {
	defer func() {
		if err != nil {
			// The websocket either failed to read, gave us an unexpected message or a message
			// that could not be decoded.  Attempt to reconnect.
			// If the reconnect fails, ErrClosed will be added to the error list allowing downstream to attempt a reconnect.
			// Otherwise, ErrClosed will not be added to the error list.
			err = errors.Join(err, ws.dial(ctx))
		}
	}()

	var (
		typ    nhws.MessageType
		reader io.Reader
	)
	typ, reader, err = ws.conn.Reader(ctx)

	if err == nil {
		if typ != nhws.MessageBinary {
			err = ErrInvalidMsgType
		} else {
			ws.decoder.Reset(reader)
			msg = &wrp.Message{}
			err = ws.decoder.Decode(msg)
		}
	}

	if err != nil {
		// The websocket gave us an unexpected message, or a message
		// that could not be decoded.  Close and send a disconnect event.
		_ = ws.conn.Close(nhws.StatusUnsupportedData, limit(err.Error()))
		ws.disconnectListeners.Visit(func(l event.DisconnectListener) {
			l.OnDisconnect(event.Disconnect{
				At:  ws.nowFunc(),
				Err: err,
			})
		})
	}

	return
}

func (ws *Websocket) dial(ctx context.Context) (err error) {
	var (
		mode = ws.nextMode()
		conn *nhws.Conn
		resp *http.Response
	)

	ws.m.Lock()
	defer ws.m.Unlock()
	defer func() {
		// Reconnect was successful, store the connection and send connect event.
		if err == nil {
			if resp.Body != nil {
				resp.Body.Close()
			}

			ws.conn = conn
			// Reset the retry policy on a successful connection.
			ws.policy = ws.retryPolicyFactory.NewPolicy(ctx)
			conn.SetReadLimit(ws.maxMessageBytes)
			ws.connectListeners.Visit(func(l event.ConnectListener) {
				l.OnConnect(event.Connect{
					Started: ws.nowFunc(),
					Mode:    mode.ToEvent(),
					At:      ws.nowFunc(),
				})
			})

			return
		}

		next, _ := ws.policy.Next()
		// Send a connect event with the error that caused the failed connection
		//  (it'll never be an auth error).
		ws.connectListeners.Visit(func(l event.ConnectListener) {
			l.OnConnect(event.Connect{
				Started:    ws.nowFunc(),
				Mode:       mode.ToEvent(),
				At:         ws.nowFunc(),
				Err:        err,
				RetryingAt: ws.nowFunc().Add(next),
			})
		})

		// Failed to reconnect, add a ErrClosed to the error list allowing downstream to attempt a reconnect.
		err = errors.Join(err, ErrClosed)
	}()

	fetchCtx, cancel := context.WithTimeout(ctx, ws.urlFetchingTimeout)
	defer cancel()

	var url string
	url, err = ws.urlFetcher(fetchCtx)
	if err != nil {
		return
	}

	// If auth fails, then continue with an openfail (no themis token) xmidt connection.
	// An auth error will never trigger an attempt to reconnect.
	if err := ws.credDecorator(ws.additionalHeaders); err != nil {
		next, _ := ws.policy.Next()
		// Send a connect event with the auth error.
		ws.connectListeners.Visit(func(l event.ConnectListener) {
			l.OnConnect(event.Connect{
				Started:    ws.nowFunc(),
				Mode:       mode.ToEvent(),
				At:         ws.nowFunc(),
				Err:        err,
				RetryingAt: ws.nowFunc().Add(next),
			})
		})
	}

	conn, resp, err = nhws.Dial(ctx, url,
		&nhws.DialOptions{
			HTTPHeader: ws.additionalHeaders,
			HTTPClient: &http.Client{
				Transport: ws.getRT(mode),
				Timeout:   ws.connectTimeout,
			},
		},
	)

	return
}

type custRT struct {
	transport http.Transport
}

func (rt *custRT) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt.transport.RoundTrip(r)
}

// getRT returns a custom RoundTripper for the WS connection.
func (ws *Websocket) getRT(mode ipMode) *custRT {

	dialer := &net.Dialer{
		Timeout:   ws.connectTimeout,
		KeepAlive: ws.keepAliveInterval,
		DualStack: false,
	}

	return &custRT{
		transport: http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, string(mode), addr)
			},
			MaxIdleConns:          1,
			MaxIdleConnsPerHost:   1,
			MaxConnsPerHost:       1,
			IdleConnTimeout:       ws.idleConnTimeout,
			TLSHandshakeTimeout:   ws.tlsHandshakeTimeout,
			ExpectContinueTimeout: ws.expectContinueTimeout,
		},
	}
}

func (ws *Websocket) nextMode() ipMode {
	if ws.mode == ipv4 && ws.withIPv6 {
		ws.mode = ipv6
	} else if ws.mode == ipv6 && ws.withIPv4 {
		ws.mode = ipv4
	}

	return ws.mode
}

func limit(s string) string {
	if len(s) > 125 {
		return s[:125]
	}
	return s
}
