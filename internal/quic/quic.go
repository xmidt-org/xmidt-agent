// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package quic

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/xmidt-org/eventor"
	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/event"
)

const (
	Name = "quic"
)

var (
	ErrMisconfiguredQuic  = errors.New("misconfigured Quic")
	ErrClosed             = errors.New("quic connection closed")
	ErrInvalidMsgType     = errors.New("invalid message type")
	ErrFromRedirectServer = errors.New("non-300 response from redirect server")
	ErrSendTimeout        = errors.New("wrp message send timed out")
	ErrHttp3NotSupported  = errors.New("http3 protocol not supported")
)

type Http3ClientConfig struct {
	QuicConfig quic.Config
	TlsConfig  tls.Config
}

// Egress interface is the egress route used to handle wrp messages that
// targets something other than this device
type Egress interface {
	// HandleWrp is called whenever a message targets something other than this device.
	HandleWrp(m wrp.Message) error
}

type QuicClient struct {
	// whether the client is enabled
	enabled bool

	// id is the device ID for the WS connection.
	id wrp.DeviceID

	// urlFetcher is the URLFetcher for the Quic connection.
	urlFetcher func(context.Context) (string, error)

	// urlFetchingTimeout is the URLFetchingTimeout for the Quic connection.
	urlFetchingTimeout time.Duration

	// credDecorator is the credentials decorator for the Quic connection.
	credDecorator func(http.Header) error

	// credDecorator is the credentials decorator for the Quic connection.
	conveyDecorator func(http.Header) error

	// sendTimeout is the send timeout for the Quic connection.
	sendTimeout time.Duration

	// keepAliveInterval is the keep alive interval for the Quic connection.
	keepAliveInterval time.Duration

	// httpClientConfig is the configuration and factory for the HTTP3 client.
	http3ClientConfig *Http3ClientConfig

	// additionalHeaders are any additional headers for the WS connection.
	additionalHeaders http.Header

	// connectListeners are the connect listeners for the WS connection.
	connectListeners eventor.Eventor[event.ConnectListener]

	// disconnectListeners are the disconnect listeners for the WS connection.
	disconnectListeners eventor.Eventor[event.DisconnectListener]

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

	conn quic.Connection

	qd Dialer
	rd Redirector

	triesSinceLastConnect atomic.Int32
}

// Option is a functional option type for Quic.
type Option interface {
	apply(*QuicClient) error
}

type optionFunc func(*QuicClient) error

func (f optionFunc) apply(c *QuicClient) error {
	return f(c)
}

func emptyDecorator(http.Header) error {
	return nil
}

// New creates a new http3 connection with the given options.
func New(opts ...Option) (*QuicClient, error) {
	qc := QuicClient{
		credDecorator:   emptyDecorator,
		conveyDecorator: emptyDecorator,
	}

	opts = append(opts,
		validateDeviceID(),
		validateURL(),
		validateCredentialsDecorator(),
		validateConveyDecorator(),
		validateNowFunc(),
		validRetryPolicy(),
	)

	for _, opt := range opts {
		if opt != nil {
			if err := opt.apply(&qc); err != nil {
				return nil, err
			}
		}
	}

	qc.credDecorator(qc.additionalHeaders)
	qc.conveyDecorator(qc.additionalHeaders)

	// separating for test purposes but introduces tramp data
	dialer := &QuicDialer{
		tlsConfig:       &qc.http3ClientConfig.TlsConfig,
		quicConfig:      qc.http3ClientConfig.QuicConfig,
		credDecorator:   qc.credDecorator,
		conveyDecorator: qc.conveyDecorator,
	}

	redirector := &UrlRedirector{
		tlsConfig:       &qc.http3ClientConfig.TlsConfig,
		quicConfig:      qc.http3ClientConfig.QuicConfig,
		credDecorator:   qc.credDecorator,
		conveyDecorator: qc.conveyDecorator,
	}
	qc.qd = dialer
	qc.rd = redirector

	return &qc, nil
}

func (qc *QuicClient) Name() string {
	return Name
}

// Start starts the http3 connection and a long running goroutine to maintain
// the connection.
func (qc *QuicClient) Start() {
	qc.triesSinceLastConnect.Store(0)

	qc.m.Lock()
	defer qc.m.Unlock()

	if qc.shutdown != nil {
		return
	}

	var ctx context.Context
	ctx, qc.shutdown = context.WithCancel(context.Background())

	go qc.run(ctx)
}

// Stop stops the quic connection.
func (qc *QuicClient) Stop() {
	qc.m.Lock()
	if qc.conn != nil {
		_ = qc.conn.CloseWithError(quic.ApplicationErrorCode(quic.ApplicationErrorErrorCode), "connection stopped by application")
	}
	shutdown := qc.shutdown
	qc.m.Unlock()

	if shutdown != nil {
		shutdown()
	}

	// run doesn't seem to exit until Stop() is finished so below
	// is deadlocked
	//qc.wg.Wait() // TODO - this is blocking forever
	qc.shutdown = nil
}

func (qc *QuicClient) IsEnabled() bool {
	return qc.enabled
}

func (qc *QuicClient) HandleWrp(m wrp.Message) error {
	return qc.Send(context.Background(), m)
}

// AddMessageListener adds a message listener to the quic connection.
// The listener will be called for every message received from the cloud.
func (qc *QuicClient) AddMessageListener(listener event.MsgListener) event.CancelFunc {
	return event.CancelFunc(qc.msgListeners.Add(listener))
}

// AddConnectListener adds a connect listener to the quic connection.
// The listener will be called every time the connection succeeds or fails
func (qc *QuicClient) AddConnectListener(listener event.ConnectListener) event.CancelFunc {
	return event.CancelFunc(qc.connectListeners.Add(listener))
}

// Send sends the provided WRP message through the existing quic connection.
func (qc *QuicClient) Send(ctx context.Context, msg wrp.Message) error {
	stream, err := qc.conn.OpenStream()
	if err != nil {
		return err
	}
	defer stream.Close()

	_, err = stream.Write(wrp.MustEncode(&msg, wrp.Msgpack))
	if err != nil {
		return err
	}

	return nil
}

func (qc *QuicClient) dial(ctx context.Context) (quic.Connection, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, qc.urlFetchingTimeout)
	defer cancel()

	fetchUrl, err := qc.urlFetcher(fetchCtx)
	if err != nil {
		return nil, err
	}

	parsedFetchUrl, err := url.Parse(fetchUrl)
	if err != nil {
		return nil, err
	}

	redirectedUrl, err := qc.rd.GetUrl(ctx, parsedFetchUrl)
	if err != nil {
		return nil, err
	}

	conn, err := qc.qd.DialQuic(ctx, redirectedUrl)

	return conn, err
}

func (qc *QuicClient) run(ctx context.Context) {
	//qc.wg.Add(1)  // see deadlock comment in Stop()
	//defer qc.wg.Done()
	defer removePrint()

	policy := qc.retryPolicyFactory.NewPolicy(ctx)

	for {
		var next time.Duration

		cEvent := event.Connect{
			Started: qc.nowFunc(),
		}

		// If auth fails, then continue with no credentials.

		conn, dialErr := qc.dial(ctx) //nolint:bodyclose
		cEvent.At = qc.nowFunc()

		if dialErr == nil {
			qc.triesSinceLastConnect.Store(0)

			qc.connectListeners.Visit(func(l event.ConnectListener) {
				l.OnConnect(cEvent)
			})

			// Reset the retry policy on a successful connection.
			policy = qc.retryPolicyFactory.NewPolicy(ctx)

			// Store the connection so writing can take place.
			qc.m.Lock()
			qc.conn = conn
			qc.m.Unlock()

			// Read loop
			for {
				var msg wrp.Message
				ctx, cancel := context.WithCancelCause(ctx)

				stream, err := conn.AcceptStream(ctx)
				if err != nil {
					cancel(nil)

					qc.conn.CloseWithError(quic.ApplicationErrorCode(quic.StreamStateError), "error accepting stream")

					dEvent := event.Disconnect{
						At:  qc.nowFunc(),
						Err: err,
					}
					qc.disconnectListeners.Visit(func(l event.DisconnectListener) {
						l.OnDisconnect(dEvent)
					})

					break
				}
				defer stream.Close()

				ctxErr := context.Cause(ctx)
				// If ctxErr is context.Canceled then the parent context has been canceled.
				if errors.Is(ctxErr, context.Canceled) {
					cancel(nil)
					break
				}

				data, err := readBytes(stream)
				if err == nil {
					decoder := wrp.NewDecoder(bytes.NewReader(data), wrp.Msgpack)
					err = decoder.Decode(&msg)
				}

				// Cancel stream context after wrp decoding.
				cancel(nil)
				if err != nil {
					// The connection gave us an unexpected message, or a message
					// that could not be decoded.  Close & reconnect.
					defer qc.conn.CloseWithError(quic.ApplicationErrorCode(quic.StreamStateError), "unable to decode wrp message")

					dEvent := event.Disconnect{
						At:  qc.nowFunc(),
						Err: err,
					}
					qc.disconnectListeners.Visit(func(l event.DisconnectListener) {
						l.OnDisconnect(dEvent)
					})

					break
				}

				qc.msgListeners.Visit(func(l event.MsgListener) {
					l.OnMessage(msg)
				})

				// close stream since we are staying in the read loop
				stream.Close()
			}
		}

		qc.triesSinceLastConnect.Add(1)

		next, _ = policy.Next()

		if dialErr != nil {
			cEvent.Err = dialErr
			cEvent.RetryingAt = qc.nowFunc().Add(next)
			cEvent.TriesSinceLastConnect = qc.triesSinceLastConnect.Load()
			qc.connectListeners.Visit(func(l event.ConnectListener) {
				l.OnConnect(cEvent)
			})
		}

		if qc.once {
			return
		}

		select {
		case <-time.After(next):
		case <-ctx.Done():
			return
		}
	}
}

// debug print for deadlock
func removePrint() {
	fmt.Println("exiting quic run")
}

func readBytes(reader io.Reader) ([]byte, error) {
	buffer := new(bytes.Buffer)
	_, err := buffer.ReadFrom(reader)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return buffer.Bytes(), nil
		}
		return nil, err
	}
	return buffer.Bytes(), nil
}
