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
	client := QuicClient{
		inactivityTimeout: time.Minute,
		credDecorator:     emptyDecorator,
		conveyDecorator:   emptyDecorator,
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
			if err := opt.apply(&client); err != nil {
				return nil, err
			}
		}
	}

	return &client, nil
}

// Start starts the http3 connection and a long running goroutine to maintain
// the connection.
func (client *QuicClient) Start() {
	client.m.Lock()
	defer client.m.Unlock()

	if client.shutdown != nil {
		return
	}

	var ctx context.Context
	ctx, client.shutdown = context.WithCancel(context.Background())

	go client.run(ctx)
}

// Stop stops the quic connection.
func (client *QuicClient) Stop() {
	client.m.Lock()
	// if ws.client != nil {
	// 	_ = ws.cl
	// }

	shutdown := client.shutdown
	client.m.Unlock()

	if shutdown != nil {
		shutdown()
	}

	client.wg.Wait()
}

func (client *QuicClient) HandleWrp(m wrp.Message) error {
	return client.Send(context.Background(), m)
}

// AddMessageListener adds a message listener to the WS connection.
// The listener will be called for every message received from the WS.
func (client *QuicClient) AddMessageListener(listener event.MsgListener) event.CancelFunc {
	return event.CancelFunc(client.msgListeners.Add(listener))
}

// Send sends the provided WRP message through the existing quic connection.
func (client *QuicClient) Send(ctx context.Context, msg wrp.Message) error {

	fmt.Println("REMOVE sending wrp response")
	stream, err := client.conn.OpenStream()
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

// create client and send headers
func (client *QuicClient) dial(ctx context.Context) (quic.Connection, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, client.urlFetchingTimeout)
	defer cancel()
	url, err := client.urlFetcher(fetchCtx)
	if err != nil {
		return nil, err
	}

	fmt.Println("in dial")

	tlsConf := &tls.Config{
		InsecureSkipVerify: true, // TODO - configure
		NextProtos:         []string{"h3"},
	}

	quicConf := &quic.Config{
		KeepAlivePeriod:      10 * time.Second,
		HandshakeIdleTimeout: 1 * time.Minute,
		MaxIdleTimeout:       1 * time.Minute,
	}

	// TODO - configure
	conn, err := quic.DialAddr(context.Background(), "localhost:4433", tlsConf, quicConf)
	if err != nil {
		fmt.Println("error dialing")
		return nil, err
	}

	roundTripper := &http3.Transport{
		TLSClientConfig: tlsConf,
		QUICConfig:      quicConf,
	}

	h3Conn := roundTripper.NewClientConn(conn)

	reqStream, err := h3Conn.OpenRequestStream(context.Background())
	if err != nil {
		fmt.Printf("error opening request stream %s", err)
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer([]byte{}))
	if err != nil {
		roundTripper.Close()
		return nil, err
	}

	req.Header.Set("Content-Type", "application/msgpack")
	client.credDecorator(req.Header)
	client.conveyDecorator(req.Header)

	// TODO
	// update client redirect to send all headers on subsequent requests
	// reqStream.CheckRedirect = func(req *http.Request, via []*http.Request) error {
	// 	// Copy headers from the first request to original requests
	// 	for key, value := range via[0].Header {
	// 		req.Header[key] = value
	// 	}
	// 	return nil
	// }

	// TODO - loop until there is no redirect, but limit redirects

	err = reqStream.SendRequestHeader(req)
	if err != nil {
		fmt.Printf("error sending request %s", err)
		return nil, err
	}

	resp, err := reqStream.ReadResponse()
	if err != nil {
		fmt.Println("error reading http3 response from server")
		return nil, err
	}

	if (resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusPermanentRedirect ) {}

	_, err = io.Copy(io.Discard, resp.Body)
	if (err != nil) && !errors.Is(err, io.EOF) {
		fmt.Printf("error reading body %s", err)
	}

	resp.Body.Close()

	return conn, err
}

func dumpContext(ctx context.Context, keys ...interface{}) {
	fmt.Println("Context Values:")
	for _, key := range keys {
		if value := ctx.Value(key); value != nil {
			fmt.Printf("  %v: %v\n", key, value)
		}
	}
}

func (client *QuicClient) run(ctx context.Context) {
	fmt.Println("in run")
	client.wg.Add(1)
	defer client.wg.Done()

	decoder := wrp.NewDecoder(nil, wrp.Msgpack)
	mode := client.nextMode(ipv4) // TODO

	policy := client.retryPolicyFactory.NewPolicy(ctx) 

	for {
		var next time.Duration

		mode = client.nextMode(mode)
		cEvent := event.Connect{
			Started: client.nowFunc(),
			Mode:    mode.ToEvent(), // TODO
		}

		// If auth fails, then continue with no credentials.
		client.credDecorator(client.additionalHeaders)
		client.conveyDecorator(client.additionalHeaders)

		conn, dialErr := client.dial(ctx) //nolint:bodyclose
		cEvent.At = client.nowFunc()

		fmt.Printf("in run after dial %s", dialErr)
		if dialErr == nil {
			client.connectListeners.Visit(func(l event.ConnectListener) {
				l.OnConnect(cEvent)
			})

			// Reset the retry policy on a successful connection.
			policy = client.retryPolicyFactory.NewPolicy(ctx)

			// Store the connection so writing can take place.
			client.m.Lock()
			client.conn = conn
			client.m.Unlock()

			// Read loop
			for {
				fmt.Println("in read loop")
				var msg wrp.Message
				//_, cancel := context.WithCancelCause(ctx)

				fmt.Println("listening for messages")

				stream, err := conn.AcceptStream(context.Background())
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
					defer client.conn.CloseWithError(quic.ApplicationErrorCode(quic.StreamStateError), "unable to decode wrp message")

					dEvent := event.Disconnect{
						At:  client.nowFunc(),
						Err: err,
					}
					client.disconnectListeners.Visit(func(l event.DisconnectListener) {
						l.OnDisconnect(dEvent)
					})

					stream.Close()

					break
				}

				client.msgListeners.Visit(func(l event.MsgListener) {
					l.OnMessage(msg)
				})
			}
		}

		fmt.Println("out of read loop")

		if client.once {
			return
		}

		next, _ = policy.Next()

		if dialErr != nil {
			fmt.Printf("dial error %s", dialErr)
			cEvent.Err = dialErr
			cEvent.RetryingAt = client.nowFunc().Add(next)
			client.connectListeners.Visit(func(l event.ConnectListener) {
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

func (ws *QuicClient) nextMode(mode ipMode) ipMode {
	if mode == ipv4 && ws.withIPv6 {
		return ipv6
	}

	if mode == ipv6 && ws.withIPv4 {
		return ipv4
	}

	return mode
}
