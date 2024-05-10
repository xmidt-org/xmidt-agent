// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/xmidt-org/eventor"
	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/wrp-go/v3"
	nhws "github.com/xmidt-org/xmidt-agent/internal/nhooyr.io/websocket"
	"github.com/xmidt-org/xmidt-agent/internal/websocket/event"
)

var (
	ErrMisconfiguredWS = errors.New("misconfigured WS")
	ErrClosed          = errors.New("websocket closed")
	ErrInvalidMsgType  = errors.New("invalid message type")
)

// Egress interface is the egress route used to handle wrp messages that
// targets something other than this device
type Egress interface {
	// HandleWrp is called whenever a message targets something other than this device.
	HandleWrp(m wrp.Message) error
}

type Websocket struct {
	// id is the device ID for the WS connection.
	id wrp.DeviceID

	// urlFetcher is the URLFetcher for the WS connection.
	urlFetcher func(context.Context) (string, error)

	// urlFetchingTimeout is the URLFetchingTimeout for the WS connection.
	urlFetchingTimeout time.Duration

	// credDecorator is the credentials decorator for the WS connection.
	credDecorator func(http.Header) error

	// credDecorator is the credentials decorator for the WS connection.
	conveyDecorator func(http.Header) error

	// pingInterval is the ping interval allowed for the WS connection.
	pingInterval time.Duration

	// pingTimeout is the ping timeout for the WS connection.
	pingTimeout time.Duration

	// sendTimeout is the send timeout for the WS connection.
	sendTimeout time.Duration

	// keepAliveInterval is the keep alive interval for the WS connection.
	keepAliveInterval time.Duration

	// client is a HTTP client used for connection attempts.
	client *http.Client

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

	conn *nhws.Conn
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
		validateConveyDecorator(),
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

	go ws.run(ctx)
}

// Stop stops the websocket connection.
func (ws *Websocket) Stop() {
	ws.m.Lock()
	if ws.conn != nil {
		ws.conn.Close(nhws.StatusNormalClosure, "")
	}

	shutdown := ws.shutdown
	ws.m.Unlock()

	if shutdown != nil {
		shutdown()
	}

	ws.wg.Wait()
}

func (ws *Websocket) HandleWrp(m wrp.Message) error {
	return ws.Send(context.Background(), m)
}

// AddMessageListener adds a message listener to the WS connection.
// The listener will be called for every message received from the WS.
func (ws *Websocket) AddMessageListener(listener event.MsgListener, cancel ...*event.CancelFunc) event.CancelFunc {
	return event.CancelFunc(ws.msgListeners.Add(listener))
}

// Send sends the provided WRP message through the existing websocket.  This
// call synchronously blocks until the write is complete.
func (ws *Websocket) Send(ctx context.Context, msg wrp.Message) error {
	err := ErrClosed
	ctx, cancel := context.WithTimeout(ctx, ws.sendTimeout)
	defer cancel()

	ws.m.Lock()
	if ws.conn != nil {
		err = ws.conn.Write(ctx, nhws.MessageBinary, wrp.MustEncode(&msg, wrp.Msgpack))
	}
	ws.m.Unlock()

	return err
}

func (ws *Websocket) run(ctx context.Context) {
	ws.wg.Add(1)
	defer ws.wg.Done()

	decoder := wrp.NewDecoder(nil, wrp.Msgpack)
	mode := ws.nextMode(ipv4)

	policy := ws.retryPolicyFactory.NewPolicy(ctx)

	for {
		var next time.Duration

		mode = ws.nextMode(mode)
		cEvent := event.Connect{
			Started: ws.nowFunc(),
			Mode:    mode.ToEvent(),
		}

		// If auth fails, then continue with openfail xmidt connection
		ws.credDecorator(ws.additionalHeaders)

		ws.conveyDecorator(ws.additionalHeaders)

		conn, _, dialErr := ws.dial(ctx, mode) //nolint:bodyclose
		cEvent.At = ws.nowFunc()

		if dialErr == nil {
			ws.connectListeners.Visit(func(l event.ConnectListener) {
				l.OnConnect(cEvent)
			})

			// Reset the retry policy on a successful connection.
			policy = ws.retryPolicyFactory.NewPolicy(ctx)

			// Store the connection so writing can take place.
			ws.m.Lock()
			ws.conn = conn
			ws.conn.SetPingListener((func(context.Context, []byte) {
				ws.heartbeatListeners.Visit(func(l event.HeartbeatListener) {
					l.OnHeartbeat(event.Heartbeat{
						At:   ws.nowFunc(),
						Type: event.PING,
					})
				})
			}))
			ws.conn.SetPongListener(func(ctx context.Context, b []byte) {
				ws.heartbeatListeners.Visit(func(l event.HeartbeatListener) {
					l.OnHeartbeat(event.Heartbeat{
						At:   ws.nowFunc(),
						Type: event.PONG,
					})
				})
			})
			ws.m.Unlock()

			// Read loop
			for {
				var msg wrp.Message
				typ, reader, err := ws.conn.Reader(ctx)
				if err == nil {
					if typ != nhws.MessageBinary {
						err = ErrInvalidMsgType
					} else {
						decoder.Reset(reader)
						err = decoder.Decode(&msg)
					}
				}

				if err != nil {
					ws.m.Lock()
					ws.conn = nil
					ws.m.Unlock()

					// The websocket gave us an unexpected message, or a message
					// that could not be decoded.  Close & reconnect.
					_ = conn.Close(nhws.StatusUnsupportedData, limit(err.Error()))

					dEvent := event.Disconnect{
						At:  ws.nowFunc(),
						Err: err,
					}
					ws.disconnectListeners.Visit(func(l event.DisconnectListener) {
						l.OnDisconnect(dEvent)
					})

					break
				}

				ws.msgListeners.Visit(func(l event.MsgListener) {
					l.OnMessage(msg)
				})
			}
		}

		if ws.once {
			return
		}

		next, _ = policy.Next()

		if dialErr != nil {
			cEvent.Err = dialErr
			cEvent.RetryingAt = ws.nowFunc().Add(next)
			ws.connectListeners.Visit(func(l event.ConnectListener) {
				l.OnConnect(cEvent)
			})
		}

		select {
		case <-time.After(next):
		case <-ctx.Done():
			return
		}
	}
}

func (ws *Websocket) dial(ctx context.Context, mode ipMode) (*nhws.Conn, *http.Response, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, ws.urlFetchingTimeout)
	defer cancel()
	url, err := ws.urlFetcher(fetchCtx)
	if err != nil {
		return nil, nil, err
	}
	ws.updateTransportDialContext(mode)
	conn, resp, err := nhws.Dial(ctx, url,
		&nhws.DialOptions{
			HTTPHeader: ws.additionalHeaders,
			HTTPClient: ws.client,
		},
	)
	if err != nil {
		return nil, resp, err
	}

	conn.SetReadLimit(ws.maxMessageBytes)
	return conn, resp, nil
}

type custRT struct {
	transport http.Transport
}

func (rt *custRT) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt.transport.RoundTrip(r)
}

// updateTransportDialContext updates Websocket's http client's RoundTripper DialContext with the given mode.
func (ws *Websocket) updateTransportDialContext(mode ipMode) {
	dialer := &net.Dialer{
		Timeout:   ws.client.Timeout,
		KeepAlive: ws.keepAliveInterval,
		DualStack: false,
	}
	ws.client.Transport.(*custRT).transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, string(mode), addr)
	}
}

func (ws *Websocket) nextMode(mode ipMode) ipMode {
	if mode == ipv4 && ws.withIPv6 {
		return ipv6
	}

	if mode == ipv6 && ws.withIPv4 {
		return ipv4
	}

	return mode
}

func limit(s string) string {
	if len(s) > 125 {
		return s[:125]
	}
	return s
}
