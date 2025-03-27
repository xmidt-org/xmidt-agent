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
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/xmidt-org/arrange/arrangehttp"
	"github.com/xmidt-org/eventor"
	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/event"
)

const (
	Name = "quic"
)

var (
	ErrMisconfiguredWS    = errors.New("misconfigured WS")
	ErrClosed             = errors.New("websocket closed")
	ErrInvalidMsgType     = errors.New("invalid message type")
	ErrFromRedirectServer = errors.New("non-200 response from redirect server")
	ErrSendTimeout        = errors.New("wrp message send timed out")
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

	// inactivityTimeout is the inactivity timeout for the Quic connection.
	// Defaults to 1 minute.
	inactivityTimeout time.Duration

	// sendTimeout is the send timeout for the Quic connection.
	sendTimeout time.Duration

	// keepAliveInterval is the keep alive interval for the Quic connection.
	keepAliveInterval time.Duration

	// httpClientConfig is the configuration and factory for the HTTP client.
	httpClientConfig arrangehttp.ClientConfig

	// httpClientConfig is the configuration and factory for the HTTP3 client.
	http3ClientConfig *Http3ClientConfig

	// additionalHeaders are any additional headers for the WS connection.
	additionalHeaders http.Header

	// maxMessageBytes is the largest allowable message to send or receive.
	maxMessageBytes int64

	// whether or not to connect directly to a quic server or redirect first
	withRedirect bool

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

	conn quic.Connection
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
	qc := QuicClient{
		inactivityTimeout: time.Minute,
		credDecorator:     emptyDecorator,
		conveyDecorator:   emptyDecorator,
	}

	opts = append(opts,
		validateDeviceID(),
		validateURL(),
		//validateIPMode(),
		validateFetchURL(),
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

	return &qc, nil
}

func (qc *QuicClient) Name() string {
	return Name
}

// Start starts the http3 connection and a long running goroutine to maintain
// the connection.
func (qc *QuicClient) Start() {
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

	qc.wg.Wait()
}

func (qc *QuicClient) IsEnabled() bool {
	return qc.enabled
}

func (qc *QuicClient) HandleWrp(m wrp.Message) error {
	fmt.Println("REMOVE sending a message")
	return qc.Send(context.Background(), m)
}

// AddMessageListener adds a message listener to the quic connection.
// The listener will be called for every message received from the cloud.
func (qc *QuicClient) AddMessageListener(listener event.MsgListener) event.CancelFunc {
	return event.CancelFunc(qc.msgListeners.Add(listener))
}

// Send sends the provided WRP message through the existing quic connection.
func (qc *QuicClient) Send(ctx context.Context, msg wrp.Message) error {
	fmt.Println("REMOVE sending wrp response")
	stream, err := qc.conn.OpenStream()
	if err != nil {
		fmt.Printf("error opening stream to client %s", err)
		return err
	}
	defer stream.Close()

	_, err = stream.Write(wrp.MustEncode(&msg, wrp.Msgpack))
	if err != nil {
		fmt.Printf("error writing to client %s", err)
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

	redirectedUrl := parsedFetchUrl
	if qc.withRedirect {
		redirectedUrl, err = qc.redirect(parsedFetchUrl)
		if err != nil {
			return nil, err
		}
	}

	fmt.Println("in dial")

	// tlsConf := &tls.Config{
	// 	InsecureSkipVerify: true, // TODO - configure
	// 	NextProtos:         []string{"h3"},
	// }

	// quicConf := &quic.Config{
	// 	KeepAlivePeriod:      10 * time.Second,
	// 	HandshakeIdleTimeout: 1 * time.Minute,
	// 	MaxIdleTimeout:       1 * time.Minute,
	// }

	conn, err := quic.DialAddr(ctx, redirectedUrl.Host, &qc.http3ClientConfig.TlsConfig, &qc.http3ClientConfig.QuicConfig)
	if err != nil {
		fmt.Println("REMOVE error dialing")
		return nil, err
	}

	roundTripper := &http3.Transport{
		TLSClientConfig: &qc.http3ClientConfig.TlsConfig,
		QUICConfig:      &qc.http3ClientConfig.QuicConfig,
	}

	h3Conn := roundTripper.NewClientConn(conn)

	reqStream, err := h3Conn.OpenRequestStream(ctx)
	if err != nil {
		fmt.Printf("REMOVE error opening request stream %s", err)
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, redirectedUrl.String(), bytes.NewBuffer([]byte{}))
	if err != nil {
		roundTripper.Close()
		return nil, err
	}

	req.Header.Set("Content-Type", "application/msgpack")
	qc.credDecorator(req.Header)
	qc.conveyDecorator(req.Header)

	err = reqStream.SendRequestHeader(req)
	if err != nil {
		fmt.Printf("REMOVE error sending request %s", err) // TODO create a specific error and wrap
		return nil, err
	}

	fmt.Println("REMOVE sent request")

	resp, err := reqStream.ReadResponse()
	if err != nil {
		fmt.Println("error reading http3 response from server")
		return nil, err
	}

	fmt.Println("REMOVE read response")

	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusPermanentRedirect {
	}

	_, err = io.Copy(io.Discard, resp.Body)
	if (err != nil) && !errors.Is(err, io.EOF) {
		fmt.Printf("error reading body %s", err)
	}

	resp.Body.Close()

	return conn, err
}

// Retrieve the url after the redirect from petasos and stop the redirect.
// Possibly temporary solution until we figure
// out how to retrieve the new connection in the client after a seamless redirect.
func (qc *QuicClient) redirect(redirectUrl *url.URL) (*url.URL, error) {
	returnUrl := redirectUrl

	config := qc.httpClientConfig
	httpClient, err := config.NewClient()
	if err != nil {
		return nil, err
	}
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		fmt.Println("REMOVE Redirected to:", req.URL.String())
		return http.ErrUseLastResponse
	}

	// httpClient := &http.Client{
	// 	Timeout: client.httpClientConfig.Timeout,
	// 	CheckRedirect: func(req *http.Request, via []*http.Request) error {
	// 		fmt.Println("REMOVE Redirected to:", req.URL.String())
	// 		return http.ErrUseLastResponse
	// 	},
	// }

	req, err := http.NewRequest(http.MethodPost, redirectUrl.String(), bytes.NewBuffer([]byte{}))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/msgpack")
	qc.credDecorator(req.Header)
	qc.conveyDecorator(req.Header)

	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Println("REMOVE Error dialing redirect host:", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		redirectedUrl, err := url.Parse(resp.Header.Get("Location"))
		if err != nil {
			fmt.Println("REMOVE Error parsing redirected URL:", err)
			return nil, err
		}
		returnUrl = redirectedUrl
	} else if resp.StatusCode >= 400 {
		errString := fmt.Sprintf("redirectServer returned status %d", resp.StatusCode)
		return nil, fmt.Errorf("%s: %w", errString, ErrFromRedirectServer)
	}

	fmt.Println("REMOVE Final URL:", returnUrl)
	return returnUrl, nil
}

func dumpContext(ctx context.Context, keys ...interface{}) {
	fmt.Println("Context Values:")
	for _, key := range keys {
		if value := ctx.Value(key); value != nil {
			fmt.Printf("  %v: %v\n", key, value)
		}
	}
}

func (qc *QuicClient) run(ctx context.Context) {
	fmt.Println("in run")
	qc.wg.Add(1)
	defer qc.wg.Done()

	decoder := wrp.NewDecoder(nil, wrp.Msgpack)

	policy := qc.retryPolicyFactory.NewPolicy(ctx)

	for {
		var next time.Duration

		cEvent := event.Connect{
			Started: qc.nowFunc(),
		}

		// If auth fails, then continue with no credentials.
		qc.credDecorator(qc.additionalHeaders)
		qc.conveyDecorator(qc.additionalHeaders)

		conn, dialErr := qc.dial(ctx) //nolint:bodyclose
		cEvent.At = qc.nowFunc()

		fmt.Printf("in run after dial %s", dialErr)
		if dialErr == nil {
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
				fmt.Println("in read loop")
				var msg wrp.Message
				//_, cancel := context.WithCancelCause(ctx)

				fmt.Println("listening for messages")

				stream, err := conn.AcceptStream(ctx)
				if err != nil {
					fmt.Println("error accepting stream")
					break
				}

				data, err := readBytes(stream)
				if err != nil {
					fmt.Println("Error reading from stream", err)
					break
				}

				fmt.Println("Client received:", string(data))
				stream.Close()

				ctxErr := context.Cause(ctx)
				err = errors.Join(err, ctxErr)
				// If ctxErr is context.Canceled then the parent context has been canceled.
				if errors.Is(ctxErr, context.Canceled) {
					//cancel(nil)
					break
				}

				if err == nil {
					decoder.Reset(bytes.NewReader(data))
					err = decoder.Decode(&msg)
				}

				// Cancel ws.conn.Reader()'s context after wrp decoding.
				//cancel(nil) // what is this doing?
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

					stream.Close()

					break
				}

				qc.msgListeners.Visit(func(l event.MsgListener) {
					l.OnMessage(msg)
				})
			}
		}

		fmt.Println("REMOVE out of read loop")

		if qc.once {
			return
		}

		next, _ = policy.Next()

		if dialErr != nil {
			fmt.Printf("dial error %s", dialErr)
			cEvent.Err = dialErr
			cEvent.RetryingAt = qc.nowFunc().Add(next)
			qc.connectListeners.Visit(func(l event.ConnectListener) {
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
