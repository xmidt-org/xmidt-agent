// SPDX-FileCopyright4yyText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package ws

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
	"github.com/xmidt-org/xmidt-agent/internal/ws/event"
	"nhooyr.io/websocket"
)

var (
	ErrMisconfiguredWS = errors.New("misconfigured WS")
)

type WS struct {
	// id is the device ID for the WS connection.
	id wrp.DeviceID

	// url is the URL for the WS connection if not using a URLFetcher, or if
	// the URLFetcher fails and fallback is allowed.
	url string

	// urlFetcher is the URLFetcher for the WS connection.
	urlFetcher func(context.Context) (string, error)

	// urlFetchingTimeout is the URLFetchingTimeout for the WS connection.
	urlFetchingTimeout time.Duration

	// allowURLFallback is whether or not to allow a fallback to the URL if
	// the URLFetcher fails.
	allowURLFallback bool

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

	// nowFunc is the now function for the WS connection.
	nowFunc func() time.Time

	// retryPolicy is the retry policy for the WS connection.
	retryPolicy retry.Policy

	m        sync.Mutex
	wg       sync.WaitGroup
	shutdown context.CancelFunc
}

// Option is a functional option type for WS.
type Option interface {
	apply(*WS) error
}

type optionFunc func(*WS) error

func (f optionFunc) apply(c *WS) error {
	return f(c)
}

// New creates a new WS connection with the given options.
func New(opts ...Option) (*WS, error) {
	var ws WS

	defaults := []Option{
		NowFunc(time.Now),
		URLFetchingTimeout(30 * time.Second),
		PingInterval(30 * time.Second),
		PingTimeout(90 * time.Second),
		ConnectTimeout(30 * time.Second),
		KeepAliveInterval(30 * time.Second),
		IdleConnTimeout(10 * time.Second),
		TLSHandshakeTimeout(10 * time.Second),
		ExpectContinueTimeout(1 * time.Second),
		WithIPv4(),
		WithIPv6(),
	}

	opts = append(defaults, opts...)

	opts = append(opts,
		validateDeviceID(),
		validateURL(),
		validateIPMode(),
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

func (ws *WS) Start() {
	ws.m.Lock()
	defer ws.m.Unlock()

	if ws.shutdown != nil {
		return
	}

	var ctx context.Context
	ctx, ws.shutdown = context.WithCancel(context.Background())

	go ws.run(ctx)
}

func (ws *WS) Stop() {
	ws.m.Lock()
	shutdown := ws.shutdown
	ws.m.Unlock()

	if shutdown != nil {
		shutdown()
	}

	ws.wg.Wait()
}

func (ws *WS) run(ctx context.Context) {
	ws.wg.Add(1)
	defer ws.wg.Done()

	mode := ws.nextMode(ipv4)

	for {
		var next time.Duration

		conn, _, err := ws.dial(ctx, mode)
		if err == nil {
		}

		if err != nil {
			next, _ = ws.retryPolicy.Next()
		}

		select {
		case <-time.After(next):
		case <-ctx.Done():
			return
		}
	}
}

func (ws *WS) dial(ctx context.Context, mode ipMode) (*websocket.Conn, *http.Response, error) {
	url, err := ws.fetchURL()
	if err != nil {
		return nil, nil, err
	}

	return websocket.Dial(ctx, url,
		&websocket.DialOptions{
			HTTPHeader: ws.additionalHeaders,
			HTTPClient: &http.Client{
				Transport: ws.getRT(mode),
				Timeout:   ws.connectTimeout,
			},
		},
	)
}

type custRT struct {
	transport http.Transport
}

func (rt *custRT) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt.transport.RoundTrip(r)
}

// getRT returns a custom RoundTripper for the WS connection.
func (ws *WS) getRT(mode ipMode) *custRT {
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

func (ws *WS) fetchURL() (string, error) {
	if ws.urlFetcher != nil {
		ctx, cancel := context.WithTimeout(context.Background(), ws.urlFetchingTimeout)
		defer cancel()
		url, err := ws.urlFetcher(ctx)
		if err == nil {
			return url, nil
		}

		if !ws.allowURLFallback {
			return "", err
		}
	}

	return ws.url, nil
}

func (ws *WS) nextMode(mode ipMode) ipMode {
	if mode == ipv4 && ws.withIPv6 {
		return ipv6
	}

	if mode == ipv6 && ws.withIPv4 {
		return ipv4
	}

	return mode
}
