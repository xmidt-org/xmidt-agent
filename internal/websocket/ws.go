// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/xmidt-org/arrange/arrangehttp"
	"github.com/xmidt-org/eventor"
	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/event"
)

const (
	Name = "websocket"
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
	// whether or not websocket protocol is enabled
	enabled bool

	// id is the device ID for the WS connection.
	id wrp.DeviceID

	// urlFetcher is the URLFetcher for the WS connection.
	urlFetcher func(context.Context) (string, error)

	// urlFetchingTimeout is the URLFetchingTimeout for the WS connection.
	urlFetchingTimeout time.Duration

	// credDecorator is the credentials decorator for the WS connection.
	credDecorator func(http.Header) error

	// conveyDecorator is the convey header decorator for the WS connection.
	conveyDecorator func(http.Header) error

	// conveyMsgDecorator is the convey msg decorator for the WS connection. Duplicates data from convey header to every message.  Should not be used.
	conveyMsgDecorator func(*wrp.Message) error

	// inactivityTimeout is the inactivity timeout for the WS connection.
	// Defaults to 1 minute.
	inactivityTimeout time.Duration

	// pingWriteTimeout is the ping timeout for the WS connection.
	pingWriteTimeout time.Duration

	// sendTimeout is the send timeout for the WS connection.
	sendTimeout time.Duration

	// keepAliveInterval is the keep alive interval for the WS connection.
	keepAliveInterval time.Duration

	// httpClientConfig is the configuration and factory for the HTTP client.
	httpClientConfig arrangehttp.ClientConfig

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
	shutdown context.CancelFunc

	conn *websocket.Conn

	triesSinceLastConnect atomic.Int32
	lastActivity          atomic.Int64 // Unix timestamp in nanoseconds
}

// Option is a functional option type for WS.
type Option interface {
	apply(*Websocket) error
}

type optionFunc func(*Websocket) error

func (f optionFunc) apply(c *Websocket) error {
	return f(c)
}

func emptyDecorator(http.Header) error {
	return nil
}

// New creates a new WS connection with the given options.
func New(opts ...Option) (*Websocket, error) {
	ws := Websocket{
		inactivityTimeout: time.Minute,
		credDecorator:     emptyDecorator,
		conveyDecorator:   emptyDecorator,
		// same default as `xmidt-agent/cmd/xmidt-agent/config.go`'s defaultConfig.Websocket.HTTPClient
		httpClientConfig: arrangehttp.ClientConfig{
			Timeout: 30 * time.Second,
			Transport: arrangehttp.TransportConfig{
				IdleConnTimeout:       10 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
	}

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

	ws.triesSinceLastConnect.Store(0)

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
		_ = ws.conn.Close(websocket.StatusNormalClosure, "")
	}

	shutdown := ws.shutdown
	ws.m.Unlock()

	if shutdown != nil {
		shutdown()
	}

	// TODO - run is not exiting until Stop() exits so this is causing deadlock
	//ws.wg.Wait()
}

func (ws *Websocket) Name() string {
	return Name
}

func (ws *Websocket) IsEnabled() bool {
	return true
}

func (ws *Websocket) HandleWrp(m wrp.Message) error {
	return ws.Send(context.Background(), m)
}

// AddMessageListener adds a message listener to the WS connection.
// The listener will be called for every message received from the WS.
func (ws *Websocket) AddMessageListener(listener event.MsgListener) event.CancelFunc {
	return event.CancelFunc(ws.msgListeners.Add(listener))
}

// AddMessageListener adds a message listener to the WS connection.
// The listener will be called for every message received from the WS.
func (ws *Websocket) AddConnectListener(listener event.ConnectListener) event.CancelFunc {
	return event.CancelFunc(ws.connectListeners.Add(listener))
}

// Send sends the provided WRP message through the existing websocket.  This
// call synchronously blocks until the write is complete.
func (ws *Websocket) Send(ctx context.Context, msg wrp.Message) error {
	err := ErrClosed
	ctx, cancel := context.WithTimeout(ctx, ws.sendTimeout)
	defer cancel()

	ws.conveyMsgDecorator(&msg)

	ws.m.Lock()
	if ws.conn != nil {
		err = ws.conn.Write(ctx, websocket.MessageBinary, wrp.MustEncode(&msg, wrp.Msgpack))
	}
	ws.m.Unlock()

	return err
}

// neither this or websocket code logs errors, this needs to be rectified.
func (ws *Websocket) run(ctx context.Context) {
	// see note in Stop()
	// ws.wg.Add(1)
	// defer ws.wg.Done()

	mode := ws.nextMode(event.Ipv4)

	policy := ws.retryPolicyFactory.NewPolicy(ctx)

	for {
		var next time.Duration

		mode = ws.nextMode(mode)
		cEvent := event.Connect{
			Started: ws.nowFunc(),
			Mode:    mode.ToEvent(),
		}

		// If auth fails, then continue with no credentials.
		ws.credDecorator(ws.additionalHeaders)

		ws.conveyDecorator(ws.additionalHeaders)

		conn, _, dialErr := ws.dial(ctx, mode) //nolint:bodyclose
		cEvent.At = ws.nowFunc()

		if dialErr == nil {
			ws.triesSinceLastConnect.Store(0)

			ws.connectListeners.Visit(func(l event.ConnectListener) {
				l.OnConnect(cEvent)
			})

			// Reset the retry policy on a successful connection.
			policy = ws.retryPolicyFactory.NewPolicy(ctx)

			// Store the connection so writing can take place.
			ws.m.Lock()
			ws.conn = conn
			ws.m.Unlock()

			// Initialize activity timestamp
			ws.lastActivity.Store(time.Now().UnixNano())

			// Create a connection-level context for this connection session
			connCtx, connCancel := context.WithCancelCause(ctx)
			defer connCancel(nil)

			// Start ping sender goroutine if pingWriteTimeout is configured
			if ws.pingWriteTimeout > 0 {
				go func() {
					// Send first ping immediately to catch timeout issues early
					pingCtx, pingCancel := context.WithTimeout(connCtx, ws.pingWriteTimeout)
					err := conn.Ping(pingCtx)
					pingCancel()
					if err != nil {
						// First ping failed - close connection
						connCancel(context.DeadlineExceeded)
						return
					}

					// Calculate ticker interval (half the timeout to stay within window)
					interval := ws.pingWriteTimeout / 2
					if interval < time.Millisecond {
						interval = time.Millisecond
					}
					ticker := time.NewTicker(interval)
					defer ticker.Stop()

					// Continue sending pings periodically
					for {
						select {
						case <-connCtx.Done():
							return
						case <-ticker.C:
							pingCtx, pingCancel := context.WithTimeout(connCtx, ws.pingWriteTimeout)
							err := conn.Ping(pingCtx)
							pingCancel()
							if err != nil {
								// Ping write failed - close connection
								connCancel(context.DeadlineExceeded)
								return
							}
						}
					}
				}()
			}

			// Monitor for inactivity using timestamp
			go func() {
				ticker := time.NewTicker(ws.inactivityTimeout / 10)
				defer ticker.Stop()

				for {
					select {
					case <-connCtx.Done():
						return
					case <-ticker.C:
						lastActivity := time.Unix(0, ws.lastActivity.Load())
						if time.Since(lastActivity) > ws.inactivityTimeout {
							connCancel(context.DeadlineExceeded)
							return
						}
					}
				}
			}()

			// Read loop
			for {
				var msg wrp.Message

				// Update activity on each read
				ws.lastActivity.Store(time.Now().UnixNano())

				typ, reader, err := ws.conn.Reader(connCtx)
				ctxErr := context.Cause(connCtx)
				err = errors.Join(err, ctxErr)
				// If ctxErr is context.Canceled then the parent context has been canceled.
				if errors.Is(ctxErr, context.Canceled) {
					break
				}

				if err == nil {
					if typ != websocket.MessageBinary {
						err = ErrInvalidMsgType
					} else {
						err = wrp.Msgpack.Decoder(reader).Decode(&msg)
					}
				}
				if err != nil {
					ws.m.Lock()
					ws.conn = nil
					ws.m.Unlock()

					// The websocket gave us an unexpected message, or a message
					// that could not be decoded.  Close & reconnect.
					_ = conn.Close(websocket.StatusUnsupportedData, limit(err.Error()))

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

		ws.triesSinceLastConnect.Add(1)

		if ws.once {
			return
		}

		next, _ = policy.Next()

		if dialErr != nil {
			cEvent.Err = dialErr
			cEvent.RetryingAt = ws.nowFunc().Add(next)
			cEvent.TriesSinceLastConnect = ws.triesSinceLastConnect.Load()
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

// createPingHandler creates a ping frame handler that updates activity tracking
// and triggers heartbeat listeners.
func (ws *Websocket) createPingHandler() func(context.Context, []byte) bool {
	return func(ctx context.Context, payload []byte) bool {
		if ctx.Err() != nil {
			return false // Don't send pong if context canceled
		}

		// Update activity timestamp
		ws.lastActivity.Store(time.Now().UnixNano())

		// Trigger heartbeat listeners
		ws.heartbeatListeners.Visit(func(l event.HeartbeatListener) {
			l.OnHeartbeat(event.Heartbeat{
				At:   ws.nowFunc(),
				Type: event.PING,
			})
		})

		return true // Send pong response
	}
}

// createPongHandler creates a pong frame handler that updates activity tracking
// and triggers heartbeat listeners.
func (ws *Websocket) createPongHandler() func(context.Context, []byte) {
	return func(ctx context.Context, payload []byte) {
		if ctx.Err() != nil {
			return // Ignore if context canceled
		}

		// Update activity timestamp
		ws.lastActivity.Store(time.Now().UnixNano())

		// Trigger heartbeat listeners
		ws.heartbeatListeners.Visit(func(l event.HeartbeatListener) {
			l.OnHeartbeat(event.Heartbeat{
				At:   ws.nowFunc(),
				Type: event.PONG,
			})
		})
	}
}

func (ws *Websocket) dial(ctx context.Context, mode event.IpMode) (*websocket.Conn, *http.Response, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, ws.urlFetchingTimeout)
	defer cancel()
	url, err := ws.urlFetcher(fetchCtx)
	if err != nil {
		return nil, nil, err
	}

	client, err := ws.newHTTPClient(mode)
	if err != nil {
		return nil, nil, err
	}

	dialOpts := &websocket.DialOptions{
		HTTPHeader: ws.additionalHeaders,
		HTTPClient: client,
	}

	// Only register ping/pong handlers if pingWriteTimeout is not configured
	// or is above a reasonable threshold. When pingWriteTimeout is very short,
	// ping operations are expected to fail and no heartbeat events should occur.
	if ws.pingWriteTimeout == 0 || ws.pingWriteTimeout >= time.Millisecond {
		dialOpts.OnPingReceived = ws.createPingHandler()
		dialOpts.OnPongReceived = ws.createPongHandler()
	}

	// Dial with callbacks configured
	conn, resp, err := websocket.Dial(ctx, url, dialOpts)
	if err != nil {
		return nil, resp, err
	}

	conn.SetReadLimit(ws.maxMessageBytes)
	// Note: SetPingWriteTimeout() doesn't exist in new library
	// Use context-based timeouts if directly calling Ping()
	return conn, resp, nil
}

type custRT struct {
	transport *http.Transport
}

func (rt *custRT) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt.transport.RoundTrip(r)
}

// newHTTPClient returns a HTTP client using the provided `mode` as its named network.
func (ws *Websocket) newHTTPClient(mode event.IpMode) (*http.Client, error) {
	config := ws.httpClientConfig
	client, err := config.NewClient()
	if err != nil {
		return nil, err
	}

	// update client redirect to send all headers on subsequent requests
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// Copy headers from the first request to original requests
		for key, value := range via[0].Header {
			req.Header[key] = value
		}
		return nil
	}

	// Override config.NewClient()'s Transport and update it's DialContext with the provided mode.
	transport, err := config.Transport.NewTransport(config.TLS)
	if err != nil {
		return nil, err
	}

	transport.Proxy = http.ProxyFromEnvironment
	dialer := &net.Dialer{
		Timeout:   client.Timeout,
		KeepAlive: ws.keepAliveInterval,
		DualStack: false,
	}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, string(mode), addr)
	}
	client.Transport = &custRT{transport: transport}

	return client, nil
}

func (ws *Websocket) nextMode(mode event.IpMode) event.IpMode {
	if mode == event.Ipv4 && ws.withIPv6 {
		return event.Ipv6
	}

	if mode == event.Ipv6 && ws.withIPv4 {
		return event.Ipv4
	}

	return mode
}

func limit(s string) string {
	if len(s) > 125 {
		return s[:125]
	}
	return s
}
