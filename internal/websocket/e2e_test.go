// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package websocket_test

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/event"
	"github.com/xmidt-org/xmidt-agent/internal/nhooyr.io/websocket"
	ws "github.com/xmidt-org/xmidt-agent/internal/websocket"
)

func TestEndToEnd(t *testing.T) {
	var finished atomic.Bool

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
					Type:        wrp.SimpleEventMessageType,
					Source:      "dns:server",
					Destination: "dns:client",
				}
				err = c.Write(ctx, websocket.MessageBinary, wrp.MustEncode(&msg, wrp.Msgpack))
				require.NoError(err)

				mt, got, err := c.Read(ctx)
				// server will halt until the websocket closes resulting in a EOF
				var closeErr websocket.CloseError
				if finished.Load() && errors.As(err, &closeErr) {
					assert.Equal(closeErr.Code, websocket.StatusNormalClosure)
					return
				}

				require.NoError(err)
				require.Equal(websocket.MessageBinary, mt)
				require.NotEmpty(got)

				err = wrp.NewDecoderBytes(got, wrp.Msgpack).Decode(&msg)
				require.NoError(err)
				require.Equal(wrp.SimpleEventMessageType, msg.Type)
				require.Equal("dns:client", msg.Source)

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
					require.Equal("dns:server", m.Source)
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
		ws.SendTimeout(90*time.Second),
		ws.FetchURLTimeout(30*time.Second),
		ws.MaxMessageBytes(256*1024),
		ws.CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ws.ConveyDecorator(func(h http.Header) error {
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

	for connectCnt.Load() == 0 {
		if ctx.Err() != nil {
			assert.Fail("timed waiting to connect")
			return
		}
	}

	got.Send(context.Background(),
		wrp.Message{
			Type:        wrp.SimpleEventMessageType,
			Source:      "dns:client",
			Destination: "dns:server",
		})

	for msgCnt.Load() == 0 {
		select {
		case <-ctx.Done():
			assert.Fail("timed out waiting for messages")
			return
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(10 * time.Millisecond)
	finished.Store(true)
	got.Stop()
	for disconnectCnt.Load() == 0 {
		select {
		case <-ctx.Done():
			assert.Fail("timed out waiting to disconnect")
			return
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
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
							require.Equal("dns:server", m.Source)
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
				ws.SendTimeout(90*time.Second),
				ws.FetchURLTimeout(30*time.Second),
				ws.MaxMessageBytes(256*1024),
				ws.CredentialsDecorator(func(h http.Header) error {
					return nil
				}),
				ws.ConveyDecorator(func(h http.Header) error {
					return nil
				}),
			)
			require.NoError(err)
			require.NotNil(got)

			got.Start()

			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			for connectCnt.Load() == 0 || disconnectCnt.Load() == 0 {
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
			for disconnectCnt.Load() == 0 {
				select {
				case <-ctx.Done():
					assert.Fail("timed out waiting to disconnect")
					return
				default:
				}
				time.Sleep(10 * time.Millisecond)
			}
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
					Type:        wrp.SimpleEventMessageType,
					Source:      "dns:server",
					Destination: "dns:client",
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
		ws.SendTimeout(90*time.Second),
		ws.FetchURLTimeout(30*time.Second),
		ws.MaxMessageBytes(256*1024),
		ws.CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ws.ConveyDecorator(func(h http.Header) error {
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
	for disconnectCnt.Load() == 0 {
		select {
		case <-ctx.Done():
			assert.Fail("timed out waiting to disconnect")
			return
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}

	assert.True(started)
	assert.True(msgCnt.Load() > 0, "got message")
}

func TestEndToEndPingWriteTimeout(t *testing.T) {
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
		ws.FetchURLTimeout(30*time.Second),
		ws.MaxMessageBytes(256*1024),
		ws.CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ws.ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		// Triggers ping timeouts
		ws.PingWriteTimeout(time.Nanosecond),
		ws.AddDisconnectListener(
			event.DisconnectListenerFunc(
				func(e event.Disconnect) {
					disconnectCnt.Add(1)
				})),
	)
	require.NoError(err)
	require.NotNil(got)

	got.Start()
	time.Sleep(500 * time.Millisecond)
	got.Stop()
	// heartbeatCnt should be zero due ping timeouts
	assert.Equal(int64(0), heartbeatCnt.Load())
	assert.Greater(connectCnt.Load(), int64(0))
	assert.Greater(disconnectCnt.Load(), int64(0))
	assert.NotEmpty(disconnectErrs)
	// disconnectErrs should only contain context.DeadlineExceeded errors
	for _, err := range disconnectErrs {
		if errors.Is(err, net.ErrClosed) {
			// net.ErrClosed may occur during tests, don't count them
			continue
		}

		assert.ErrorIs(err, context.DeadlineExceeded)
	}

}

func TestEndToEndInactivityTimeout(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5000*time.Millisecond)
	defer cancel()

	var (
		disconnectCnt atomic.Int64
	)

	s := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				c, err := websocket.Accept(w, r, nil)
				require.NoError(err)
				defer c.CloseNow()

				ctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
				defer cancel()
				mt, got, err := c.Read(ctx)

				assert.ErrorIs(err, io.EOF)
				assert.Equal(websocket.MessageType(0), mt)
				assert.Nil(got)
			}))
	defer s.Close()

	got, err := ws.New(
		ws.URL(s.URL),
		ws.DeviceID("mac:112233445566"),
		ws.RetryPolicy(&retry.Config{
			Interval:    time.Second,
			Multiplier:  2.0,
			Jitter:      1.0 / 3.0,
			MaxInterval: 341*time.Second + 333*time.Millisecond,
		}),
		ws.WithIPv4(),
		ws.NowFunc(time.Now),
		ws.FetchURLTimeout(30*time.Second),
		ws.MaxMessageBytes(256*1024),
		ws.CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ws.ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		// Triggers inactivity timeouts
		ws.InactivityTimeout(10*time.Millisecond),
		ws.AddDisconnectListener(
			event.DisconnectListenerFunc(
				func(e event.Disconnect) {
					disconnectCnt.Add(1)
				})),
	)
	require.NoError(err)
	require.NotNil(got)

	got.Start()
	time.Sleep(400 * time.Millisecond)
	got.Stop()
	for disconnectCnt.Load() == 0 {
		select {
		case <-ctx.Done():
			assert.Fail("timed out waiting to disconnect")
			return
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
}
