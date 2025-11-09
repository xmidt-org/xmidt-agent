// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"context"

	"crypto/tls"

	"errors"
	"net/http/httptest"

	"fmt"

	"net/http"

	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/suite"
	qc "github.com/xmidt-org/xmidt-agent/internal/quic"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"

	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/event"
	ws "github.com/xmidt-org/xmidt-agent/internal/websocket"
)

type key string

const (
	QuicConnectionKey key = "quicConnection"
	ShouldRedirectKey key = "shouldRedirect"
	SuiteKey          key = "suite"
)

func GetWrpMessage(origin string) wrp.Message {
	return wrp.Message{
		Type:        wrp.SimpleEventMessageType,
		Source:      fmt.Sprintf("event:test.com/%s", origin),
		Destination: "mac:4ca161000109/mock_config",
		PartnerIDs:  []string{"foobar"},
	}
}

type EToESuite struct {
	suite.Suite
}

func TestEToESuite(t *testing.T) {
	suite.Run(t, new(EToESuite))
}

func (suite *EToESuite) SetupSuite() {}

func (suite *EToESuite) TearDownTest() {
}

func (suite *EToESuite) TestSwitchFromQuicToWebsocket() {
	var finished bool

	s := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				c, err := websocket.Accept(w, r, nil)
				suite.NoError(err)
				defer c.CloseNow()

				ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
				defer cancel()

				msg := wrp.Message{
					Type:        wrp.SimpleEventMessageType,
					Source:      "dns:server",
					Destination: "dns:client",
				}
				err = c.Write(ctx, websocket.MessageBinary, wrp.MustEncode(&msg, wrp.Msgpack))
				suite.NoError(err)

				mt, got, err := c.Read(context.Background())
				// server will halt until the websocket closes resulting in a EOF
				var closeErr websocket.CloseError
				if finished && errors.As(err, &closeErr) {
					suite.Equal(closeErr.Code, websocket.StatusNormalClosure)
					return
				}

				suite.NoError(err)
				suite.Equal(websocket.MessageBinary, mt)
				suite.NotEmpty(got)

				msg = wrp.Message{}
				err = wrp.NewDecoderBytes(got, wrp.Msgpack).Decode(&msg)
				suite.NoError(err)
				suite.Equal(wrp.SimpleEventMessageType, msg.Type)
				suite.Equal("event:test.com/websocket", msg.Source)

				c.Close(websocket.StatusNormalClosure, "")
			}))

	defer s.Close()

	testId := "one"

	var qcConnectCnt, qcDisconnectCnt atomic.Int64

	quic, err := qc.New(
		qc.Enabled(true),
		qc.URL(s.URL),
		qc.DeviceID("mac:112233445566"),
		qc.HTTP3Client(&qc.Http3ClientConfig{
			QuicConfig: quic.Config{
				KeepAlivePeriod: 500 * time.Millisecond,
			},
			TlsConfig: tls.Config{
				NextProtos:         []string{"h3"},
				InsecureSkipVerify: true, // #nosec G402
			},
		}),
		qc.AddConnectListener(
			event.ConnectListenerFunc(
				func(event.Connect) {
					qcConnectCnt.Add(1)
				})),
		qc.AddDisconnectListener(
			event.DisconnectListenerFunc(
				func(event.Disconnect) {
					qcDisconnectCnt.Add(1)
				})),
		qc.RetryPolicy(&retry.Config{
			Interval:    10 * time.Millisecond,
			Multiplier:  2.0,
			Jitter:      1.0 / 3.0,
			MaxInterval: 341*time.Second + 333*time.Millisecond,
		}),
		qc.NowFunc(time.Now),
		qc.SendTimeout(90*time.Second),
		qc.FetchURLTimeout(30*time.Second),
		qc.CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		qc.ConveyDecorator(func(h http.Header) error {
			h.Add("testId", testId)
			return nil
		}),
		qc.ConveyMsgDecorator(func(m *wrp.Message) error {
			return nil
		}),
	)
	suite.NoError(err)
	suite.NotNil(quic)

	var wsMsgCnt, wsConnectCnt, wsDisconnectCnt atomic.Int64

	websocket, err := ws.New(
		ws.URL(s.URL),
		ws.DeviceID("mac:112233445566"),
		ws.AddMessageListener(
			event.MsgListenerFunc(
				func(m wrp.Message) {
					suite.Equal(wrp.SimpleEventMessageType, m.Type)
					suite.Equal("dns:server", m.Source)
					wsMsgCnt.Add(1)
				})),
		ws.AddConnectListener(
			event.ConnectListenerFunc(
				func(event.Connect) {
					wsConnectCnt.Add(1)
				})),
		ws.AddDisconnectListener(
			event.DisconnectListenerFunc(
				func(event.Disconnect) {
					wsDisconnectCnt.Add(1)
				})),
		ws.RetryPolicy(&retry.Config{
			Interval:    time.Second,
			Multiplier:  2.0,
			Jitter:      1.0 / 3.0,
			MaxInterval: 341*time.Second + 333*time.Millisecond,
		}),
		ws.WithIPv4(),
		ws.NowFunc(time.Now),
		ws.SendTimeout(90*time.Second),
		ws.FetchURLTimeout(30*time.Second),
		ws.MaxMessageBytes(256*1024),
		ws.CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ws.ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		ws.ConveyMsgDecorator(func(m *wrp.Message) error {
			return nil
		}),
	)
	suite.NoError(err)
	suite.NotNil(websocket)

	cloud, err := New(
		QuicClient(quic),
		Websocket(websocket),
		PreferQuic(true),
		MaxTries(1),
	)
	suite.NoError(err)

	cloud.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	for {
		if wsConnectCnt.Load() < 1 {
			time.Sleep(10 * time.Millisecond)
		} else {
			break
		}
		if ctx.Err() != nil {
			suite.Fail("timed out waiting to connect")
			return
		}
	}

	//suite.m.Lock()
	//suite.True(postsReceivedFromClient[testId])
	//suite.m.Unlock()

	wrpHandler := cloud.(wrpkit.Handler)
	wrpHandler.HandleWrp(GetWrpMessage(cloud.Name())) // TODO - first one is not received
	time.Sleep(10 * time.Millisecond)
	//got.Send(context.Background(), GetWrpMessage("client"))

	// verify client receives message from server
	for {
		if wsMsgCnt.Load() < 1 {
			time.Sleep(10 * time.Millisecond)
		} else {
			break
		}
		if ctx.Err() != nil {
			suite.Fail("timed out waiting for message from server")
			return
		}
	}

	// verify quic tried to connect max times before failing
	suite.Equal(int64(2), qcConnectCnt.Load())

	time.Sleep(10 * time.Millisecond)

	cloud.Stop()
	finished = true

	for {
		if wsDisconnectCnt.Load() < 1 {
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
