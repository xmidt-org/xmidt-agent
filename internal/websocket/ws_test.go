// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/event"
	mockevent "github.com/xmidt-org/xmidt-agent/internal/mocks/event"
)

var (
	errUnknown = errors.New("unknown error")
)

func TestNew(t *testing.T) {
	fetcher := func(context.Context) (string, error) {
		return "http://example.com/url", nil
	}

	wsDefaults := []Option{
		WithIPv6(),
	}
	tests := []struct {
		description string
		opts        []Option
		expectedErr error
		check       func(*assert.Assertions, *Websocket)
		checks      []func(*assert.Assertions, *Websocket)
		optStr      string
	}{
		{
			description: "nil option",
			expectedErr: errUnknown,
		}, {
			description: "common config",
			opts: append(
				wsDefaults,
				FetchURL(fetcher),
				DeviceID("mac:112233445566"),
				Enabled(true),
				AdditionalHeaders(map[string][]string{
					"some-other-header": {"vAlUE"},
				}),
				CredentialsDecorator(func(h http.Header) error {
					h.Add("Credentials-Decorator", "some value")
					return nil
				}),
				ConveyDecorator(func(h http.Header) error {
					h.Add("Convey-Decorator", "some value")
					return nil
				}),
				ConveyMsgDecorator(func(w *wrp.Message) error {
					return nil
				}),
				NowFunc(time.Now),
				RetryPolicy(retry.Config{}),
			),
			check: func(assert *assert.Assertions, c *Websocket) {
				assert.True(c.enabled)
				// URL Related
				assert.Equal("mac:112233445566", string(c.id))
				assert.NotNil(c.urlFetcher)
				u, err := c.urlFetcher(context.Background())
				assert.NoError(err)
				assert.Equal("http://example.com/url", u)

				// Headers
				assert.NotNil(c.additionalHeaders)
				assert.NoError(c.credDecorator(c.additionalHeaders))
				assert.NoError(c.conveyDecorator(c.additionalHeaders))
				assert.Equal("mac:112233445566", c.additionalHeaders.Get("X-Webpa-Device-Name"))
				assert.Equal("vAlUE", c.additionalHeaders.Get("Some-Other-Header"))
				assert.Equal("some value", c.additionalHeaders.Get("Credentials-Decorator"))
				assert.Equal("some value", c.additionalHeaders.Get("Convey-Decorator"))
				assert.Equal("websocket", c.Name())
			},
		},

		// URL Related
		{
			description: "no url, or fetcher",
			opts: []Option{
				DeviceID("mac:112233445566"),
			},
			expectedErr: errUnknown,
		}, {
			description: "fetcher",
			opts: append(
				wsDefaults,
				DeviceID("mac:112233445566"),
				FetchURL(fetcher),
				CredentialsDecorator(func(h http.Header) error {
					return nil
				}),
				ConveyDecorator(func(h http.Header) error {
					return nil
				}),
				NowFunc(time.Now),
				RetryPolicy(retry.Config{}),
			),
			check: func(assert *assert.Assertions, c *Websocket) {
				u, err := c.urlFetcher(context.Background())
				assert.NoError(err)
				assert.Equal("http://example.com/url", u)
			},
		},

		// IP Mode Related
		{
			description: "no allowed ip modes",
			opts: []Option{
				DeviceID("mac:112233445566"),
				URL("http://example.com"),
				WithIPv4(false),
				WithIPv6(false),
			},
			expectedErr: errUnknown,
		},

		// Boundary testing for options
		{
			description: "negative url fetching timeout",
			opts: []Option{
				FetchURLTimeout(-1),
			},
			expectedErr: ErrMisconfiguredWS,
		}, {
			description: "negative inactivity timeout",
			opts: []Option{
				InactivityTimeout(-1),
			},
			expectedErr: ErrMisconfiguredWS,
		}, {
			description: "negative ping write timeout",
			opts: []Option{
				PingWriteTimeout(-1),
			},
			expectedErr: ErrMisconfiguredWS,
		},
		{
			description: "nil convey decorator",
			opts: []Option{
				ConveyDecorator(nil),
			},
			expectedErr: ErrMisconfiguredWS,
		},
		{
			description: "nil convey msg decorator",
			opts: []Option{
				ConveyMsgDecorator(nil),
			},
			expectedErr: ErrMisconfiguredWS,
		},

		// Test the now func option
		{
			description: "custom now func",
			opts: append(
				wsDefaults,
				URL("http://example.com"),
				DeviceID("mac:112233445566"),
				NowFunc(func() time.Time {
					return time.Unix(1234, 0)
				}),
				CredentialsDecorator(func(h http.Header) error {
					return nil
				}),
				ConveyDecorator(func(h http.Header) error {
					return nil
				}),
				RetryPolicy(retry.Config{}),
			),
			check: func(assert *assert.Assertions, c *Websocket) {
				if assert.NotNil(c.nowFunc) {
					assert.Equal(time.Unix(1234, 0), c.nowFunc())
				}
			},
		}, {
			description: "nil now func",
			opts: []Option{
				NowFunc(nil),
			},
			expectedErr: ErrMisconfiguredWS,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			got, err := New(tc.opts...)

			checks := append(tc.checks, tc.check)
			for _, check := range checks {
				if check != nil {
					check(assert, got)
				}
			}

			if tc.expectedErr == nil {
				assert.NotNil(got)
				assert.NoError(err)
				return
			}

			assert.Nil(got)
			assert.Error(err)

			if !errors.Is(tc.expectedErr, errUnknown) {
				assert.ErrorIs(err, tc.expectedErr)
			}
		})
	}
}

func TestMessageListener(t *testing.T) {
	assert := assert.New(t)

	var m mockevent.MockListeners

	m.On("OnMessage", mock.Anything).Return()

	got, err := New(
		URL("http://example.com"),
		DeviceID("mac:112233445566"),
		AddMessageListener(&m),
		WithIPv6(),
		CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		NowFunc(time.Now),
		RetryPolicy(retry.Config{}),
	)

	// external resource can call this after New
	got.AddMessageListener(
		event.MsgListenerFunc(
			func(m wrp.Message) {
				fmt.Println("do something with message")
			}))

	assert.NoError(err)
	if assert.NotNil(got) {
		got.msgListeners.Visit(func(l event.MsgListener) {
			l.OnMessage(wrp.Message{})
		})
		m.AssertExpectations(t)
	}
}

func TestConnectListener(t *testing.T) {
	assert := assert.New(t)

	var m mockevent.MockListeners

	m.On("OnConnect", mock.Anything).Return()

	got, err := New(
		URL("http://example.com"),
		DeviceID("mac:112233445566"),
		AddConnectListener(&m),
		WithIPv6(),
		CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		ConveyMsgDecorator(func(m *wrp.Message) error {
			return nil
		}),
		NowFunc(time.Now),
		RetryPolicy(retry.Config{}),
	)

	// called by external actors after New
	got.AddConnectListener(
		event.ConnectListenerFunc(
			func(e event.Connect) {
				fmt.Println("do something after connect event")
			}))

	assert.NoError(err)
	if assert.NotNil(got) {
		got.connectListeners.Visit(func(l event.ConnectListener) {
			l.OnConnect(event.Connect{})
		})
		m.AssertExpectations(t)
	}
}

func TestDisconnectListener(t *testing.T) {
	assert := assert.New(t)

	var m mockevent.MockListeners

	m.On("OnDisconnect", mock.Anything).Return()

	got, err := New(
		URL("http://example.com"),
		DeviceID("mac:112233445566"),
		AddDisconnectListener(&m),
		WithIPv6(),
		CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		NowFunc(time.Now),
		RetryPolicy(retry.Config{}),
	)

	assert.NoError(err)
	if assert.NotNil(got) {
		got.disconnectListeners.Visit(func(l event.DisconnectListener) {
			l.OnDisconnect(event.Disconnect{})
		})
		m.AssertExpectations(t)
	}
}

func TestHeartbeatListener(t *testing.T) {
	assert := assert.New(t)

	var m mockevent.MockListeners

	m.On("OnHeartbeat", mock.Anything).Return()

	got, err := New(
		URL("http://example.com"),
		DeviceID("mac:112233445566"),
		AddHeartbeatListener(&m),
		WithIPv6(),
		CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		NowFunc(time.Now),
		RetryPolicy(retry.Config{}),
	)

	assert.NoError(err)
	if assert.NotNil(got) {
		got.heartbeatListeners.Visit(func(l event.HeartbeatListener) {
			l.OnHeartbeat(event.Heartbeat{})
		})
		m.AssertExpectations(t)
	}
}

func TestNextMode(t *testing.T) {
	defaults := []Option{
		CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		NowFunc(time.Now),
		RetryPolicy(retry.Config{}),
	}
	tests := []struct {
		description string
		opts        []Option
		mode        event.IpMode
		expected    event.IpMode
	}{
		{
			description: "IPv4 to IPv6",
			mode:        event.Ipv4,
			expected:    event.Ipv6,
			opts: append(defaults,
				WithIPv6(true),
				WithIPv4(true),
			),
		}, {
			description: "IPv6 to IPv4",
			mode:        event.Ipv6,
			expected:    event.Ipv4,
			opts: append(defaults,
				WithIPv6(true),
				WithIPv4(true),
			),
		}, {
			description: "IPv4 to IPv4",
			opts: append(defaults,
				WithIPv4(true),
				WithIPv6(false),
			),
			mode:     event.Ipv4,
			expected: event.Ipv4,
		}, {
			description: "IPv6 to IPv6",
			opts: append(defaults,
				WithIPv4(false),
				WithIPv6(true),
			),
			mode:     event.Ipv6,
			expected: event.Ipv6,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			opts := append(tc.opts,
				DeviceID("mac:112233445566"),
				URL("http://example.com"),
			)
			got, err := New(opts...)
			require.NoError(err)
			require.NotNil(got)
			assert.Equal(tc.expected, got.nextMode(tc.mode))
		})
	}
}

func TestLimit(t *testing.T) {
	tests := []struct {
		description string
		in          string
		want        string
	}{
		{
			description: "short",
			in:          "short",
			want:        "short",
		}, {
			description: "long",
			in:          "----------------------------------------------------------------------------------------------------------------------------------",
			want:        "-----------------------------------------------------------------------------------------------------------------------------",
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			got := limit(tc.in)
			assert.Equal(tc.want, got)
		})
	}
}

func Test_emptyDecorator(t *testing.T) {
	assert.NoError(t, emptyDecorator(http.Header{}))
}

func Test_CancelCtx(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	s := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				c, err := websocket.Accept(w, r, nil)
				require.NoError(err)
				defer c.CloseNow()

				mt, got, err := c.Read(context.Background())

				assert.ErrorIs(err, io.EOF)
				assert.Equal(websocket.MessageType(0), mt)
				assert.Nil(got)
			}))
	defer s.Close()

	got, err := New(
		URL(s.URL),
		DeviceID("mac:112233445566"),
		RetryPolicy(&retry.Config{
			Interval:    time.Second,
			Multiplier:  2.0,
			Jitter:      1.0 / 3.0,
			MaxInterval: 341*time.Second + 333*time.Millisecond,
		}),
		WithIPv4(),
		NowFunc(time.Now),
		FetchURLTimeout(30*time.Second),
		MaxMessageBytes(256*1024),
		CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		// Triggers inactivity timeouts
		InactivityTimeout(time.Hour),
	)
	require.NoError(err)
	require.NotNil(got)

	got.Start()
	time.Sleep(500 * time.Millisecond)
	assert.Equal(int32(0), got.triesSinceLastConnect.Load())
	got.shutdown()
	time.Sleep(500 * time.Millisecond)
	got.Stop()

}
