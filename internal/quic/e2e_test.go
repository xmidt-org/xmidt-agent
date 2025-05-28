// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package quic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"log"

	"fmt"
	"math/big"
	"net"
	"net/http"

	"sync/atomic"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/stretchr/testify/suite"

	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/event"
)

type key string

const (
	QuicConnectionKey key = "quicConnection"
	ShouldRedirectKey key = "shouldRedirect"
	SuiteKey          key = "suite"
)

type myHandler struct{}

var (
	remoteServerPort           = "4433"
	redirectServerPort         = "4432"
)

func (h myHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("in ServeHTTP")

	rc := http.NewResponseController(w)
    defer rc.Flush()

	conn := r.Context().Value(QuicConnectionKey).(quic.Connection)
	suite := r.Context().Value(SuiteKey).(*EToESuite)
	shouldRedirect := r.Context().Value(ShouldRedirectKey).(bool)
	testId := r.Header.Get("testId")

	fmt.Println(r.Header)

	if shouldRedirect {
		suite.clientRedirected = true
		fmt.Println("about to redirect")
		http.Redirect(w, r, fmt.Sprintf("https://127.0.0.1:%s", remoteServerPort), http.StatusMovedPermanently)
		w.Write([]byte("test body"))
		fmt.Println("redirected")
		return
	}

	suite.postReceivedFromClient = true

	w.WriteHeader(http.StatusOK)

	w.Header().Set("Content-Type", "text/plain")

	_, err := fmt.Fprint(w, "Thanks for the post! Love, the server.")
	if err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
		return
	}

	go sendMessageFromServer(conn, suite, context.Background())
	go listenForMessageFromClient(conn, suite, context.Background(), testId)
}

func sendMessageFromServer(conn quic.Connection, suite *EToESuite, ctx context.Context) {
	msg := GetWrpMessage("server")

	stream, err := conn.OpenStream()
	if err != nil {
		log.Println("Stream open error:", err)
		return
	}

	_, err = stream.Write(wrp.MustEncode(&msg, wrp.Msgpack))
	if err != nil {
		fmt.Println(err)
		return
	}

	stream.Close()
}

func listenForMessageFromClient(conn quic.Connection, suite *EToESuite, ctx context.Context, testId string) error {
	stream, err := conn.AcceptStream(ctx)
	if err != nil {
		fmt.Printf("error accepting stream  %s", err)
		fmt.Println(err)
	}
	defer stream.Close()

	buf, err := readBytes(stream)
	if err != nil {
		fmt.Println("Error reading:", err)
		return err
	}

	fmt.Println("stream handled got: " + string(buf))

	suite.messageReceivedFromClient = true

	return nil
}

func GetWrpMessage(origin string) wrp.Message {
	return wrp.Message{
		Type:        wrp.SimpleEventMessageType,
		Source:      fmt.Sprintf("event:test.com/%s", origin),
		Destination: "mac:4ca161000109/mock_config",
		PartnerIDs:  []string{"foobar"},
	}
}

func (suite *EToESuite) StartRemoteServer(port string, redirect bool) {
	tlsConf := generateTLSConfig()
	tlsConf = http3.ConfigureTLSConfig(tlsConf)
	quicConf := &quic.Config{
		KeepAlivePeriod:      500 * time.Millisecond,
		HandshakeIdleTimeout: 1 * time.Minute,
		MaxIdleTimeout:       2 * time.Minute,
	}

	h := myHandler{}

	server := &http3.Server{
		Addr:       fmt.Sprintf(":%s", port),
		TLSConfig:  tlsConf,
		Handler:    h,
		QUICConfig: quicConf,
		ConnContext: func(ctx context.Context, c quic.Connection) context.Context {
			ctx = context.WithValue(ctx, QuicConnectionKey, c)
			ctx = context.WithValue(ctx, SuiteKey, suite)
			ctx = context.WithValue(ctx, ShouldRedirectKey, redirect)
			return ctx
		},
		
	}

	if (redirect) {
		suite.redirectServer = server
	} else {
		suite.server = server
	}

	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("0.0.0.0:%s", port))
	if err != nil {
		fmt.Println("error resolving udp address")
		log.Fatal(err)
	}

	remoteAddr := udpAddr.String()

	fmt.Println(remoteAddr)

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Println("error dialing udp")
		log.Fatal(err)
	}

	tr := quic.Transport{
		Conn: conn,
	}

	ln, err := tr.ListenEarly(tlsConf, quicConf)
	if err != nil {
		fmt.Printf("error listening early %s", err)
		log.Fatal(err)
	}

	fmt.Println("listened early")

	for {
		fmt.Println("about to wait for connections")
		c, err := ln.Accept(context.Background())
		if err != nil {
			fmt.Printf("error accepting connection %s", err)
			continue
		}

		fmt.Printf("accepted connection on port %s\n", port)

		switch c.ConnectionState().TLS.NegotiatedProtocol {
		case http3.NextProtoH3:
			fmt.Println("got http3 request")
			server.ServeQUICConn(c)
			fmt.Println("handled connection")
			fmt.Println(c.Context())
		}
	}
}

func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"h3"},
	}
}

type EToESuite struct {
	suite.Suite
	clientRedirected bool
	postReceivedFromClient bool
	messageReceivedFromClient bool
	server *http3.Server
	redirectServer *http3.Server

}

func TestEToESuite(t *testing.T) {
	suite.Run(t, new(EToESuite))
}

func (suite *EToESuite) SetupSuite() {
	go suite.StartRemoteServer(redirectServerPort, true)
	go suite.StartRemoteServer(remoteServerPort, false)

	time.Sleep(10 * time.Second)
}

func (suite *EToESuite) TearDownSuite() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	suite.server.Shutdown(ctx)
	suite.redirectServer.Shutdown(ctx)
}

func (suite *EToESuite) TestEndToEnd() {
	testId := "one"

	var msgCnt, connectCnt, disconnectCnt atomic.Int64

	got, err := New(
		Enabled(true),
		URL(fmt.Sprintf("https://127.0.0.1:%s", redirectServerPort)),
		DeviceID("mac:112233445566"),
		HTTP3Client(&Http3ClientConfig{
			QuicConfig: quic.Config{
				KeepAlivePeriod: 500 * time.Millisecond,
			},
			TlsConfig: tls.Config{
				NextProtos:         []string{"h3"},
				InsecureSkipVerify: true,
			},
		}),
		AddMessageListener(
			event.MsgListenerFunc(
				func(m wrp.Message) {
					fmt.Println("xmidt-agent got message")
					suite.Equal(wrp.SimpleEventMessageType, m.Type)
					suite.Equal("event:test.com/server", m.Source)
					msgCnt.Add(1)
				})),
		AddConnectListener(
			event.ConnectListenerFunc(
				func(event.Connect) {
					connectCnt.Add(1)
				})),
		AddDisconnectListener(
			event.DisconnectListenerFunc(
				func(event.Disconnect) {
					disconnectCnt.Add(1)
				})),
		RetryPolicy(&retry.Config{
			Interval:    time.Second,
			Multiplier:  2.0,
			Jitter:      1.0 / 3.0,
			MaxInterval: 341*time.Second + 333*time.Millisecond,
		}),
		NowFunc(time.Now),
		SendTimeout(90*time.Second),
		FetchURLTimeout(30*time.Second),
		CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ConveyDecorator(func(h http.Header) error {
			h.Add("testId", testId)
			return nil
		}),
	)
	suite.NoError(err)
	suite.NotNil(got)

	got.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for {
		if connectCnt.Load() < 1 {
			time.Sleep(10 * time.Millisecond)
		} else {
			break
		}
		if ctx.Err() != nil {
			suite.Fail("timed out waiting to connect")
			return
		}
	}

	suite.True(suite.clientRedirected)
	suite.True(suite.postReceivedFromClient)

	got.Send(context.Background(), GetWrpMessage("client")) // TODO - first one is not received
	time.Sleep(10 * time.Millisecond)
	got.Send(context.Background(), GetWrpMessage("client"))

	// verify client receives message from server
	for {
		if msgCnt.Load() < 1 {
			time.Sleep(10 * time.Millisecond)
		} else {
			break
		}
		if ctx.Err() != nil {
			suite.Fail("timed out waiting for message from server")
			return
		}
	}

	time.Sleep(10 * time.Millisecond)

	suite.True(suite.messageReceivedFromClient)

	got.Stop()

	for {
		if disconnectCnt.Load() < 1 {
			time.Sleep(10 * time.Millisecond)
		} else {
			break
		}
		if ctx.Err() != nil {
			suite.Fail("timed out waiting to disconnect")
			return
		}
	}
}
