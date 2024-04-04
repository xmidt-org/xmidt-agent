// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package websocket_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/websocket"
	"github.com/xmidt-org/xmidt-agent/internal/websocket/event"
	wsh "github.com/xmidt-org/xmidt-agent/internal/wrphandlers/websocket"
	nhws "nhooyr.io/websocket"
)

func TestHandler_HandleWrp(t *testing.T) {
	var msgReceivedCnt, connectCnt, disconnectCnt atomic.Int64

	opts := []websocket.Option{
		websocket.DeviceID("mac:112233445566"),
		websocket.AddConnectListener(
			event.ConnectListenerFunc(
				func(event.Connect) {
					connectCnt.Add(1)
				})),
		websocket.AddDisconnectListener(
			event.DisconnectListenerFunc(
				func(event.Disconnect) {
					disconnectCnt.Add(1)
				})),
		websocket.RetryPolicy(&retry.Config{
			Interval:    time.Second,
			Multiplier:  2.0,
			Jitter:      1.0 / 3.0,
			MaxInterval: 341*time.Second + 333*time.Millisecond,
		}),
		websocket.WithIPv4(),
		websocket.NowFunc(time.Now),
		websocket.ConnectTimeout(30 * time.Second),
		websocket.FetchURLTimeout(30 * time.Second),
		websocket.MaxMessageBytes(256 * 1024),
		websocket.CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
	}
	tests := []struct {
		description string
		msg         wrp.Message
	}{
		// response cases
		{
			description: "response case SimpleRequestResponseMessageType",
			msg:         wrp.Message{Type: wrp.SimpleRequestResponseMessageType, Source: "server"},
		},
		{
			description: "response case CreateMessageType",
			msg:         wrp.Message{Type: wrp.CreateMessageType, Source: "server"},
		},
		{
			description: "response case RetrieveMessageType",
			msg:         wrp.Message{Type: wrp.RetrieveMessageType, Source: "server"},
		},
		{
			description: "response case UpdateMessageType",
			msg:         wrp.Message{Type: wrp.UpdateMessageType, Source: "server"},
		},
		{
			description: "response case DeleteMessageType",
			msg:         wrp.Message{Type: wrp.DeleteMessageType, Source: "server"},
		},
		// No response cases
		{
			description: "no response case AuthorizationMessageType",
			msg:         wrp.Message{Type: wrp.AuthorizationMessageType, Source: "server"},
		},
		{
			description: "no response case SimpleEventMessageType",
			msg:         wrp.Message{Type: wrp.SimpleEventMessageType, Source: "server"},
		},
		{
			description: "no response case ServiceRegistrationMessageType",
			msg:         wrp.Message{Type: wrp.ServiceRegistrationMessageType, Source: "server"},
		},
		{
			description: "no response case ServiceAliveMessageType",
			msg:         wrp.Message{Type: wrp.ServiceAliveMessageType, Source: "server"},
		},
		{
			description: "no response case UnknownMessageType",
			msg:         wrp.Message{Type: wrp.UnknownMessageType, Source: "server"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			msgReceivedCnt.Swap(0)
			connectCnt.Swap(0)
			disconnectCnt.Swap(0)
			s := httptest.NewServer(
				http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						c, err := nhws.Accept(w, r, nil)
						require.NoError(err)
						defer func() {
							c.Close(nhws.StatusNormalClosure, "")
						}()
						defer c.CloseNow()

						ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
						defer cancel()

						err = c.Write(ctx, nhws.MessageBinary, wrp.MustEncode(&tc.msg, wrp.Msgpack))
						require.NoError(err)

						mt, buf, err := c.Read(ctx)
						// server will halt until the websocket closes resulting in a EOF
						var closeErr nhws.CloseError
						if errors.As(err, &closeErr) {
							assert.Equal(closeErr.Code, nhws.StatusNormalClosure)
							return
						} else if !tc.msg.Type.RequiresTransaction() && assert.ErrorIs(err, context.DeadlineExceeded) {
							return
						}

						require.NoError(err)
						assert.Equal(nhws.MessageBinary, mt)
						assert.NotEmpty(buf)

						msg := wrp.Message{}
						err = wrp.NewDecoderBytes(buf, wrp.Msgpack).Decode(&msg)
						require.NoError(err)
						assert.Equal("client", msg.Source)
						assert.True(msg.Type.RequiresTransaction())
						msgReceivedCnt.Add(1)

					}))

			var wrpHandler *wsh.Handler

			testOpts := append(opts,
				websocket.URL(s.URL),
				websocket.AddMessageListener(
					event.MsgListenerFunc(
						func(m wrp.Message) {
							assert.Equal("server", m.Source)
							require.NotNil(wrpHandler)

							m.Source = "client"
							assert.NoError(wrpHandler.HandleWrp(m))
						})))
			ws, err := websocket.New(testOpts...)
			require.NoError(err)
			require.NotNil(ws)

			wrpHandler, err = wsh.New(ws)
			require.NoError(err)
			require.NotNil(wrpHandler)

			ws.Start()

			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			for {
				if !tc.msg.Type.RequiresTransaction() || (connectCnt.Load() > 0 && disconnectCnt.Load() > 0) {
					break
				}
				select {
				case <-ctx.Done():
					assert.Fail("timed out waiting for messages")
					return
				default:
				}
				time.Sleep(10 * time.Millisecond)
			}

			<-ctx.Done()
			if tc.msg.Type.RequiresTransaction() {
				assert.Equal(1, int(msgReceivedCnt.Load()))
			} else {
				assert.Zero(msgReceivedCnt.Load())
			}

			ws.Stop()
			s.Close()
		})
	}
}
