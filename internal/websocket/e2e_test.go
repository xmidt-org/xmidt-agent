// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package websocket_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/nhooyr.io/websocket"
	ws "github.com/xmidt-org/xmidt-agent/internal/websocket"
	"github.com/xmidt-org/xmidt-agent/internal/websocket/event"
)

func TestEndToEnd(t *testing.T) {
	var finished bool

	assert := assert.New(t)
	require := require.New(t)

	s := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				c, err := websocket.Accept(w, r, nil)
				require.NoError(err)
				defer c.CloseNow()

				ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
				defer cancel()

				msg := wrp.Message{
					Type:   wrp.SimpleEventMessageType,
					Source: "server",
				}
				err = c.Write(ctx, websocket.MessageBinary, wrp.MustEncode(&msg, wrp.Msgpack))
				require.NoError(err)

				mt, got, err := c.Read(ctx)
				// server will halt until the websocket closes resulting in a EOF
				var closeErr websocket.CloseError
				if finished && errors.As(err, &closeErr) {
					assert.Equal(closeErr.Code, websocket.StatusNormalClosure)
					return
				}

				require.NoError(err)
				require.Equal(websocket.MessageBinary, mt)
				require.NotEmpty(got)

				err = wrp.NewDecoderBytes(got, wrp.Msgpack).Decode(&msg)
				require.NoError(err)
				require.Equal(wrp.SimpleEventMessageType, msg.Type)
				require.Equal("client", msg.Source)

				c.Close(websocket.StatusNormalClosure, "")
			}))
	defer s.Close()

	var msgCnt, connectCnt, disconnectCnt atomic.Int64

	got, err := ws.New(
		ws.URL(s.URL),
		ws.DeviceID("mac:112233445566"),
		ws.AddMessageListener(
			event.MsgListenerFunc(
				func(m wrp.Message) {
					require.Equal(wrp.SimpleEventMessageType, m.Type)
					require.Equal("server", m.Source)
					msgCnt.Add(1)
				})),
		ws.AddConnectListener(
			event.ConnectListenerFunc(
				func(event.Connect) {
					connectCnt.Add(1)
				})),
		ws.AddDisconnectListener(
			event.DisconnectListenerFunc(
				func(event.Disconnect) {
					disconnectCnt.Add(1)
				})),
		ws.RetryPolicy(&retry.Config{
			Interval:    time.Second,
			Multiplier:  2.0,
			Jitter:      1.0 / 3.0,
			MaxInterval: 341*time.Second + 333*time.Millisecond,
		}),
		ws.WithIPv4(),
		ws.NowFunc(time.Now),
		ws.ConnectTimeout(30*time.Second),
		ws.FetchURLTimeout(30*time.Second),
		ws.MaxMessageBytes(256*1024),
		ws.CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
	)
	require.NoError(err)
	require.NotNil(got)

	got.Start()

	// Allow multiple calls to start.
	got.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	for {
		if msgCnt.Load() < 1 {
			time.Sleep(10 * time.Millisecond)
		} else {
			break
		}
		if ctx.Err() != nil {
			assert.Fail("timed out waiting for messages")
			return
		}
	}

	got.Send(context.Background(),
		wrp.Message{
			Type:   wrp.SimpleEventMessageType,
			Source: "client",
		})

	for {
		if msgCnt.Load() > 0 && connectCnt.Load() > 0 && disconnectCnt.Load() > 0 {
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
	time.Sleep(10 * time.Millisecond)
	finished = true
	got.Stop()
}

func TestEndToEndBadData(t *testing.T) {
	tests := []struct {
		description string
		typ         websocket.MessageType
		data        []byte
	}{
		{
			description: "invalid data",
			typ:         websocket.MessageBinary,
			data:        []byte{0x99, 0x86},
		}, {
			description: "invalid data type",
			typ:         websocket.MessageText,
			data:        []byte{0x99, 0x86},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {

			assert := assert.New(t)
			require := require.New(t)

			s := httptest.NewServer(
				http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						c, err := websocket.Accept(w, r, nil)
						require.NoError(err)
						defer c.CloseNow()

						ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
						defer cancel()

						err = c.Write(ctx, tc.typ, tc.data)
						require.NoError(err)

						_, _, err = c.Read(ctx)
						require.Error(err)
					}))
			defer s.Close()

			var msgCnt, connectCnt, disconnectCnt atomic.Int64

			got, err := ws.New(
				ws.URL(s.URL),
				ws.DeviceID("mac:112233445566"),
				ws.Once(),
				ws.AddMessageListener(
					event.MsgListenerFunc(
						func(m wrp.Message) {
							require.Equal(wrp.SimpleEventMessageType, m.Type)
							require.Equal("server", m.Source)
							msgCnt.Add(1)
						})),
				ws.AddConnectListener(
					event.ConnectListenerFunc(
						func(event.Connect) {
							connectCnt.Add(1)
						})),
				ws.AddDisconnectListener(
					event.DisconnectListenerFunc(
						func(event.Disconnect) {
							disconnectCnt.Add(1)
						})),
				ws.RetryPolicy(&retry.Config{
					Interval:       50 * time.Millisecond,
					Multiplier:     2.0,
					MaxElapsedTime: 300 * time.Millisecond,
				}),
				ws.WithIPv4(),
				ws.NowFunc(time.Now),
				ws.ConnectTimeout(30*time.Second),
				ws.FetchURLTimeout(30*time.Second),
				ws.MaxMessageBytes(256*1024),
				ws.CredentialsDecorator(func(h http.Header) error {
					return nil
				}),
			)
			require.NoError(err)
			require.NotNil(got)

			got.Start()

			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			for {
				if connectCnt.Load() > 0 && disconnectCnt.Load() > 0 {
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
			time.Sleep(10 * time.Millisecond)
			got.Stop()
		})
	}
}

func TestEndToEndConnectionIssues(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// don't start the server until after the 3rd attempt.

	s := httptest.NewUnstartedServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				c, err := websocket.Accept(w, r, nil)
				require.NoError(err)
				defer c.CloseNow()

				ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
				defer cancel()

				msg := wrp.Message{
					Type:   wrp.SimpleEventMessageType,
					Source: "server",
				}
				err = c.Write(ctx, websocket.MessageBinary, wrp.MustEncode(&msg, wrp.Msgpack))
				require.NoError(err)
			}))

	var msgCnt, connectCnt, disconnectCnt atomic.Int64

	var mutex sync.Mutex

	got, err := ws.New(
		ws.FetchURL(func(context.Context) (string, error) {
			mutex.Lock()
			defer mutex.Unlock()
			if s.URL == "" {
				return "", fmt.Errorf("no url")
			}
			return s.URL, nil
		}),
		ws.DeviceID("mac:112233445566"),
		ws.AddMessageListener(
			event.MsgListenerFunc(
				func(m wrp.Message) {
					msgCnt.Add(1)
				})),
		ws.AddConnectListener(
			event.ConnectListenerFunc(
				func(e event.Connect) {
					connectCnt.Add(1)
				})),
		ws.AddDisconnectListener(
			event.DisconnectListenerFunc(
				func(event.Disconnect) {
					disconnectCnt.Add(1)
				})),
		ws.RetryPolicy(&retry.Config{
			Interval:       50 * time.Millisecond,
			Multiplier:     2.0,
			MaxElapsedTime: 300 * time.Millisecond,
		}),
		ws.WithIPv4(),
		ws.NowFunc(time.Now),
		ws.ConnectTimeout(30*time.Second),
		ws.FetchURLTimeout(30*time.Second),
		ws.MaxMessageBytes(256*1024),
		ws.CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
	)
	require.NoError(err)
	require.NotNil(got)

	got.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
	defer cancel()

	var started bool
	for {
		if connectCnt.Load() >= 3 && !started {
			mutex.Lock()
			s.Start()
			mutex.Unlock()
			defer func() {
				mutex.Lock()
				s.Close()
				mutex.Unlock()
			}()
			started = true
		}
		if disconnectCnt.Load() > 0 {
			break
		}

		select {
		case <-ctx.Done():
			assert.Fail("timed out waiting for messages")
			return
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
	got.Stop()

	assert.True(started)
	assert.True(msgCnt.Load() > 0, "got message")
}

func TestEndToEndPingTimeout(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	s := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				c, err := websocket.Accept(w, r, nil)
				require.NoError(err)
				defer c.CloseNow()

				assert.Error(c.Ping(context.Background()))
			}))
	defer s.Close()

	var (
		connectCnt, disconnectCnt, heartbeatCnt atomic.Int64
		got                                     *ws.Websocket
		err                                     error
		disconnectErrs                          []error
	)
	got, err = ws.New(
		ws.URL(s.URL),
		ws.DeviceID("mac:112233445566"),
		ws.AddHeartbeatListener(
			event.HeartbeatListenerFunc(
				func(event.Heartbeat) {
					heartbeatCnt.Add(1)
				})),
		ws.AddConnectListener(
			event.ConnectListenerFunc(
				func(event.Connect) {
					connectCnt.Add(1)
				})),
		ws.AddDisconnectListener(
			event.DisconnectListenerFunc(
				func(e event.Disconnect) {
					disconnectErrs = append(disconnectErrs, e.Err)
					disconnectCnt.Add(1)
				})),
		ws.RetryPolicy(&retry.Config{
			Interval:    time.Second,
			Multiplier:  2.0,
			Jitter:      1.0 / 3.0,
			MaxInterval: 341*time.Second + 333*time.Millisecond,
		}),
		ws.WithIPv4(),
		ws.NowFunc(time.Now),
		ws.ConnectTimeout(30*time.Second),
		ws.FetchURLTimeout(30*time.Second),
		ws.MaxMessageBytes(256*1024),
		ws.CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		// Trigger a ping timeout
		ws.PingTimeout(time.Nanosecond),
	)
	require.NoError(err)
	require.NotNil(got)

	got.Start()
	time.Sleep(500 * time.Millisecond)
	got.Stop()
	// heartbeatCnt should be zero due to a ping timeout
	assert.Equal(int64(0), heartbeatCnt.Load())
	assert.Greater(connectCnt.Load(), int64(0))
	assert.Greater(disconnectCnt.Load(), int64(0))
	// disconnectErrs should only contain
	assert.NotEmpty(disconnectErrs)
	// All disconnectErrs errors should be caused by context.DeadlineExceeded
	for _, err := range disconnectErrs {
		if errors.Is(err, net.ErrClosed) {
			// net.ErrClosed may occur during tests, don't count them
			continue
		}

		assert.ErrorIs(err, context.DeadlineExceeded)
	}

}
