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
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/xmidt-org/arrange/arrangehttp"
	"github.com/xmidt-org/eventor"
	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/wrp-go/v3"
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

type QuicClient struct {
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
	wg       sync.WaitGroup
	shutdown context.CancelFunc

	client *http.Client
}

// Option is a functional option type for WS.
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
	roundTripper := &http3.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"h3"},
		},
		QUICConfig: &quic.Config{
			KeepAlivePeriod: 10 * time.Second,
		},
	}

	ws := QuicClient{
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
		client: &http.Client{
			Transport: roundTripper,
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

// Start starts the http3 connection and a long running goroutine to maintain
// the connection.
func (ws *QuicClient) Start() {
	ws.m.Lock()
	defer ws.m.Unlock()

	if ws.shutdown != nil {
		return
	}

	var ctx context.Context
	ctx, ws.shutdown = context.WithCancel(context.Background())

	go ws.run(ctx)
}

// Stop stops the quic connection.
func (ws *QuicClient) Stop() {
	ws.m.Lock()
	// if ws.client != nil {
	// 	_ = ws.cl
	// }

	shutdown := ws.shutdown
	ws.m.Unlock()

	if shutdown != nil {
		shutdown()
	}

	ws.wg.Wait()
}

func (ws *QuicClient) HandleWrp(m wrp.Message) error {
	return ws.Send(context.Background(), m)
}

// AddMessageListener adds a message listener to the WS connection.
// The listener will be called for every message received from the WS.
func (ws *QuicClient) AddMessageListener(listener event.MsgListener) event.CancelFunc {
	return event.CancelFunc(ws.msgListeners.Add(listener))
}

// Send sends the provided WRP message through the existing websocket.  This
// call synchronously blocks until the write is complete.
func (ws *QuicClient) Send(ctx context.Context, msg wrp.Message) error {
	// TODO - configure url
	// fetchCtx, cancel := context.WithTimeout(ctx, ws.urlFetchingTimeout)
	// defer cancel()
	// url, err := ws.urlFetcher(fetchCtx)
	// if err != nil {
	// 	return nil, nil, err
	// }

	req, err := http.NewRequest(http.MethodPost, "https://localhost:4433", bytes.NewBuffer(wrp.MustEncode(&msg, wrp.Msgpack)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/msgpack")

	// Send request
	resp, err := ws.client.Do(req)
	if err != nil {
		return err
	}

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// create client and send headers
func (ws *QuicClient) dial(ctx context.Context) (*http.Client, *http.Response, error) {
	// fetchCtx, cancel := context.WithTimeout(ctx, ws.urlFetchingTimeout)
	// defer cancel()
	// url, err := ws.urlFetcher(fetchCtx)
	// if err != nil {
	// 	return nil, nil, err
	// }

	// fake usage
	fmt.Println(ctx)

	client, err := ws.newHTTPClient()
	if err != nil {
		return nil, nil, err
	}

	// TODO configure
	req, err := http.NewRequest(http.MethodPost, "https://localhost:4433", bytes.NewBuffer([]byte{}))
	if err != nil {
		return client, nil, err
	}
	req.Header.Set("Content-Type", "application/msgpack")
	ws.credDecorator(req.Header)
	ws.conveyDecorator(req.Header)

	// Send request
	resp, err := ws.client.Do(req)
	if err != nil {
		return client, resp, err
	}

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return client, resp, err
	}

	return client, resp, nil
}

func (ws *QuicClient) run(ctx context.Context) {
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

		// If auth fails, then continue with no credentials.
		ws.credDecorator(ws.additionalHeaders)

		ws.conveyDecorator(ws.additionalHeaders)

		client, _, dialErr := ws.dial(ctx) //nolint:bodyclose
		cEvent.At = ws.nowFunc()

		if dialErr == nil {
			ws.connectListeners.Visit(func(l event.ConnectListener) {
				l.OnConnect(cEvent)
			})

			// Reset the retry policy on a successful connection.
			policy = ws.retryPolicyFactory.NewPolicy(ctx)

			// Store the connection so writing can take place.
			ws.m.Lock()
			ws.client = client
			//activity := make(chan struct{})  // this is probably not needed, only used by ping and pong
			ws.m.Unlock()

			// Read loop
			for {
				var msg wrp.Message
				ctx, cancel := context.WithCancelCause(ctx)

				// Monitor for activity.
				go func() {
					//inactivityTimeout := time.After(ws.inactivityTimeout)
				loop1:
					for {
						select {
						case <-ctx.Done():
							break loop1
							// case <-activity:
							// 	inactivityTimeout = time.After(ws.inactivityTimeout)
							// case <-inactivityTimeout:
							// 	// inactivityTimeout occurred, cancel the context.
							// 	cancel(context.DeadlineExceeded)
							// 	break loop1
						}
					}
				}()

				resp, err := client.Get("https://localhost:4433")
				ctxErr := context.Cause(ctx)
				err = errors.Join(err, ctxErr)
				// If ctxErr is context.Canceled then the parent context has been canceled.
				if errors.Is(ctxErr, context.Canceled) {
					cancel(nil)
					break
				}

				if err == nil {
					decoder.Reset(resp.Body)
					//err = decoder.Decode(&msg)  // TODO - just for testing
					msg = wrp.Message{}  // TODO - temp
				}

				// Cancel ws.conn.Reader()'s context after wrp decoding.
				cancel(nil)
				if err != nil {
					// The connection gave us an unexpected message, or a message
					// that could not be decoded.  Close & reconnect.
					defer ws.client.Transport.(*http3.Transport).Close()

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


// newHTTPClient returns a HTTP client using the provided `mode` as its named network.
func (ws *QuicClient) newHTTPClient() (*http.Client, error) {
	roundTripper := &http3.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"h3"},
		},
		QUICConfig: &quic.Config{
			KeepAlivePeriod: 10 * time.Second,
		},
	}

	client := &http.Client{
		Transport: roundTripper,
	}

	// update client redirect to send all headers on subsequent requests
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// Copy headers from the first request to original requests
		for key, value := range via[0].Header {
			req.Header[key] = value
		}
		return nil
	}

	return client, nil
}

func (ws *QuicClient) nextMode(mode ipMode) ipMode {
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
