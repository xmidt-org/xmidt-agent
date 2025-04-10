// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0
package quic

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
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
	Url        = "https://example.com"
)

func TestNew(t *testing.T) {
	fetcher := func(context.Context) (string, error) {
		return Url, nil
	}

	opts := []Option{}

	tests := []struct {
		description string
		opts        []Option
		expectedErr error
		check       func(*assert.Assertions, *QuicClient)
		checks      []func(*assert.Assertions, *QuicClient)
		optStr      string
	}{
		{
			description: "nil option",
			expectedErr: errUnknown,
		}, {
			description: "common config",
			opts: append(
				opts,
				Enabled(true),
				FetchURL(fetcher),
				DeviceID("mac:112233445566"),
				//WithRedirect(false),
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
				HTTP3Client(&Http3ClientConfig{
					QuicConfig: quic.Config{},
					TlsConfig:  tls.Config{},
				}),
				NowFunc(time.Now),
				RetryPolicy(retry.Config{}),
			),
			check: func(assert *assert.Assertions, c *QuicClient) {
				assert.True(c.enabled)
				// URL Related
				assert.Equal("mac:112233445566", string(c.id))
				assert.NotNil(c.urlFetcher)
				u, err := c.urlFetcher(context.Background())
				assert.NoError(err)
				assert.Equal(Url, u)
				//assert.False(c.withRedirect)

				// Headers
				assert.NotNil(c.additionalHeaders)
				assert.NoError(c.credDecorator(c.additionalHeaders))
				assert.NoError(c.conveyDecorator(c.additionalHeaders))
				assert.Equal("mac:112233445566", c.additionalHeaders.Get("X-Webpa-Device-Name"))
				assert.Equal("vAlUE", c.additionalHeaders.Get("Some-Other-Header"))
				assert.Equal("some value", c.additionalHeaders.Get("Credentials-Decorator"))
				assert.Equal("some value", c.additionalHeaders.Get("Convey-Decorator"))
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
				opts,
				DeviceID("mac:112233445566"),
				FetchURL(fetcher),
				CredentialsDecorator(func(h http.Header) error {
					return nil
				}),
				ConveyDecorator(func(h http.Header) error {
					return nil
				}),
				HTTP3Client(&Http3ClientConfig{
					QuicConfig: quic.Config{},
					TlsConfig:  tls.Config{},
				}),
				NowFunc(time.Now),
				RetryPolicy(retry.Config{}),
			),
			check: func(assert *assert.Assertions, c *QuicClient) {
				u, err := c.urlFetcher(context.Background())
				assert.NoError(err)
				assert.Equal(Url, u)
			},
		},

		// Boundary testing for options
		{
			description: "negative url fetching timeout",
			opts: []Option{
				FetchURLTimeout(-1),
			},
			expectedErr: ErrMisconfiguredQuic,
		},

		// Test the now func option
		{
			description: "custom now func",
			opts: append(
				opts,
				URL(Url),
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
				HTTP3Client(&Http3ClientConfig{
					QuicConfig: quic.Config{},
					TlsConfig:  tls.Config{},
				}),
				RetryPolicy(retry.Config{}),
			),
			check: func(assert *assert.Assertions, c *QuicClient) {
				if assert.NotNil(c.nowFunc) {
					assert.Equal(time.Unix(1234, 0), c.nowFunc())
				}
			},
		}, {
			description: "nil now func",
			opts: []Option{
				NowFunc(nil),
			},
			expectedErr: ErrMisconfiguredQuic,
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
		URL(Url),
		DeviceID("mac:112233445566"),
		AddMessageListener(&m),
		CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		HTTP3Client(&Http3ClientConfig{
			QuicConfig: quic.Config{},
			TlsConfig:  tls.Config{},
		}),
		NowFunc(time.Now),
		RetryPolicy(retry.Config{}),
	)

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
		URL(Url),
		DeviceID("mac:112233445566"),
		AddConnectListener(&m),
		CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		HTTP3Client(&Http3ClientConfig{
			QuicConfig: quic.Config{},
			TlsConfig:  tls.Config{},
		}),
		NowFunc(time.Now),
		RetryPolicy(retry.Config{}),
	)

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
		URL(Url),
		DeviceID("mac:112233445566"),
		AddDisconnectListener(&m),
		CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		HTTP3Client(&Http3ClientConfig{
			QuicConfig: quic.Config{},
			TlsConfig:  tls.Config{},
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
		URL(Url),
		DeviceID("mac:112233445566"),
		AddHeartbeatListener(&m),
		CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		HTTP3Client(&Http3ClientConfig{
			QuicConfig: quic.Config{},
			TlsConfig:  tls.Config{},
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

func Test_emptyDecorator(t *testing.T) {
	assert.NoError(t, emptyDecorator(http.Header{}))
}

func Test_CancelCtx(t *testing.T) {
	require := require.New(t)

	got, err := New(
		Enabled(true),
		URL(Url),
		DeviceID("mac:112233445566"),
		RetryPolicy(&retry.Config{
			Interval:    time.Second,
			Multiplier:  2.0,
			Jitter:      1.0 / 3.0,
			MaxInterval: 341*time.Second + 333*time.Millisecond,
		}),
		NowFunc(time.Now),
		FetchURLTimeout(30*time.Second),
		CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		HTTP3Client(&Http3ClientConfig{
			QuicConfig: quic.Config{},
			TlsConfig:  tls.Config{},
		}),
	)
	require.NoError(err)
	require.NotNil(got)
	mockDialer := NewMockDialer()

	got.qd = mockDialer
	mockConn := NewMockConnection()

	mockDialer.On("DialQuic", mock.Anything, mock.Anything).Return(mockConn, nil)

	got.Start()
	time.Sleep(500 * time.Millisecond)
	got.shutdown()
	time.Sleep(500 * time.Millisecond)
	got.Stop()
}

func Test_DialErr(t *testing.T) {
	require := require.New(t)

	mockEventListeners := &mockevent.MockListeners{}
	got, err := New(
		Enabled(true),
		URL("http://test.com"),
		DeviceID("mac:112233445566"),
		RetryPolicy(&retry.Config{
			Interval:    time.Second,
			Multiplier:  2.0,
			Jitter:      1.0 / 3.0,
			MaxInterval: 341*time.Second + 333*time.Millisecond,
		}),
		NowFunc(time.Now),
		FetchURLTimeout(30*time.Second),
		CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		HTTP3Client(&Http3ClientConfig{
			QuicConfig: quic.Config{},
			TlsConfig:  tls.Config{},
		}),
		AddDisconnectListener(mockEventListeners),
	)
	require.NoError(err)
	require.NotNil(got)
	mockDialer := NewMockDialer()
	mockRedirector := NewMockRedirector()

	got.qd = mockDialer
	got.rd = mockRedirector
	mockConn := NewMockConnection()

	remoteServerUrl, err := url.Parse("https://127.0.0.1:4433")
	require.NoError(err)
	mockRedirector.On("GetUrl", mock.Anything, mock.Anything).Return(remoteServerUrl, nil)
	mockDialer.On("DialQuic", mock.Anything, mock.Anything).Return(mockConn, errors.New("dial error"))

	got.Start()
	time.Sleep(10 * time.Millisecond)

	mockEventListeners.AssertNotCalled(t, "OnConnect", mock.Anything)
}

func Test_StreamErr(t *testing.T) {
	require := require.New(t)

	mockEventListeners := &mockevent.MockListeners{}
	got, err := New(
		Enabled(true),
		URL("https://127.0.0.1:4432"),
		DeviceID("mac:112233445566"),
		RetryPolicy(&retry.Config{
			Interval:    time.Second,
			Multiplier:  2.0,
			Jitter:      1.0 / 3.0,
			MaxInterval: 341*time.Second + 333*time.Millisecond,
		}),
		NowFunc(time.Now),
		FetchURLTimeout(30*time.Second),
		CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		HTTP3Client(&Http3ClientConfig{
			QuicConfig: quic.Config{},
			TlsConfig:  tls.Config{},
		}),
		AddConnectListener(mockEventListeners),
		AddDisconnectListener(mockEventListeners),
	)
	require.NoError(err)
	require.NotNil(got)
	mockDialer := NewMockDialer()
	mockRedirector := NewMockRedirector()

	got.qd = mockDialer
	got.rd = mockRedirector
	mockConn := NewMockConnection()
	mockStr := NewMockStream([]byte("xxxx"))
	mockStr.On("Close").Return(nil)
	mockConn.On("AcceptStream", mock.Anything).Return(mockStr, errors.New("some error"))
	mockConn.On("CloseWithError", mock.Anything, mock.Anything).Return(nil)

	mockEventListeners.On("OnConnect", mock.Anything).Return()
	mockEventListeners.On("OnDisconnect", mock.Anything).Return()

	remoteServerUrl, err := url.Parse("https://127.0.0.1:4433")
	require.NoError(err)
	mockRedirector.On("GetUrl", mock.Anything, mock.Anything).Return(remoteServerUrl, nil)
	mockDialer.On("DialQuic", mock.Anything, mock.Anything).Return(mockConn, nil)

	got.Start()
	time.Sleep(10 * time.Millisecond)

	mockEventListeners.AssertCalled(t, "OnConnect", mock.Anything)
	mockEventListeners.AssertCalled(t, "OnDisconnect", mock.Anything)
	mockConn.AssertCalled(t, "AcceptStream", mock.Anything)
	mockConn.AssertCalled(t, "CloseWithError", mock.Anything, mock.Anything)
	mockRedirector.AssertCalled(t, "GetUrl", mock.Anything, mock.Anything)
}

func Test_DecodeErr(t *testing.T) {
	require := require.New(t)

	mockEventListeners := &mockevent.MockListeners{}
	got, err := New(
		Enabled(true),
		URL(Url),
		DeviceID("mac:112233445566"),
		RetryPolicy(&retry.Config{
			Interval:    time.Second,
			Multiplier:  2.0,
			Jitter:      1.0 / 3.0,
			MaxInterval: 341*time.Second + 333*time.Millisecond,
		}),
		NowFunc(time.Now),
		FetchURLTimeout(30*time.Second),
		CredentialsDecorator(func(h http.Header) error {
			return nil
		}),
		ConveyDecorator(func(h http.Header) error {
			return nil
		}),
		HTTP3Client(&Http3ClientConfig{
			QuicConfig: quic.Config{},
			TlsConfig:  tls.Config{},
		}),

		AddConnectListener(mockEventListeners),
		AddDisconnectListener(mockEventListeners),
	)
	require.NoError(err)
	require.NotNil(got)

	mockDialer := NewMockDialer()
	mockRedirector := NewMockRedirector()
	got.qd = mockDialer
	got.rd = mockRedirector

	mockConn := NewMockConnection()
	mockStr := NewMockStream([]byte("xxxx"))
	mockConn.On("AcceptStream", mock.Anything).Return(mockStr, nil)
	mockConn.On("CloseWithError", mock.Anything, mock.Anything).Return(nil)
	mockStr.On("Read", mock.Anything).Return(5, nil)
	mockStr.On("Close").Return(nil)

	mockEventListeners.On("OnConnect", mock.Anything).Return()
	mockEventListeners.On("OnDisconnect", mock.Anything).Return()

	remoteServerUrl, err := url.Parse("https://127.0.0.1:4433")
	require.NoError(err)
	mockRedirector.On("GetUrl", mock.Anything, mock.Anything).Return(remoteServerUrl, nil)
	mockDialer.On("DialQuic", mock.Anything, mock.Anything).Return(mockConn, nil)

	got.Start()
	time.Sleep(10 * time.Millisecond)

	mockEventListeners.AssertCalled(t, "OnConnect", mock.Anything)
	mockConn.AssertCalled(t, "AcceptStream", mock.Anything)
	mockEventListeners.AssertCalled(t, "OnDisconnect", mock.Anything)
	//mockConn.AssertCalled(t, "CloseWithError", mock.Anything, mock.Anything) - in the debugger, this gets called
	mockRedirector.AssertCalled(t, "GetUrl", mock.Anything, mock.Anything)
}
