// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package quic

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/event"
	mockevent "github.com/xmidt-org/xmidt-agent/internal/mocks/event"
)

var (
	errUnknown      = errors.New("unknown error")
	RemoteServerUrl = "https://127.0.0.1:4433"
)

func TestNew(t *testing.T) {
	fetcher := func(context.Context) (string, error) {
		return RemoteServerUrl, nil
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
				SendTimeout(1*time.Second),
				KeepAliveInterval(5*time.Second),
				NowFunc(time.Now),
				RetryPolicy(retry.Config{}),
				Once(false),
			),
			check: func(assert *assert.Assertions, c *QuicClient) {
				assert.True(c.enabled)
				// URL Related
				assert.Equal("mac:112233445566", string(c.id))
				assert.NotNil(c.urlFetcher)
				u, err := c.urlFetcher(context.Background())
				assert.NoError(err)
				assert.Equal(RemoteServerUrl, u)
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
		{
			description: "empty device id",
			opts: []Option{
				DeviceID(""),
			},
			expectedErr: ErrMisconfiguredQuic,
		},
		{
			description: "empty url",
			opts: []Option{
				URL(""),
			},
			expectedErr: ErrMisconfiguredQuic,
		},
		{
			description: "nil url fetcher",
			opts: []Option{
				FetchURL(nil),
			},
			expectedErr: ErrMisconfiguredQuic,
		},
		{
			description: "nil credentials decorator",
			opts: []Option{
				CredentialsDecorator(nil),
			},
			expectedErr: ErrMisconfiguredQuic,
		},
		{
			description: "nil convey decorator",
			opts: []Option{
				ConveyDecorator(nil),
			},
			expectedErr: ErrMisconfiguredQuic,
		},
		{
			description: "negative keep alive",
			opts: []Option{
				KeepAliveInterval(-1),
			},
			expectedErr: ErrMisconfiguredQuic,
		},
		{
			description: "negative send timeout",
			opts: []Option{
				SendTimeout(-1),
			},
			expectedErr: ErrMisconfiguredQuic,
		},
		{
			description: "nil retry policy",
			opts: []Option{
				RetryPolicy(nil),
			},
			expectedErr: ErrMisconfiguredQuic,
		},
		{
			description: "no quic address",
			opts: []Option{
				DeviceID("mac:112233445566"),
			},
			expectedErr: ErrMisconfiguredQuic,
		},
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
				assert.Equal(RemoteServerUrl, u)
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
				URL(RemoteServerUrl),
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
		URL(RemoteServerUrl),
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
		URL(RemoteServerUrl),
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

	// called by external actors after New
	got.AddConnectListener(
		event.ConnectListenerFunc(
			func(e event.Connect) {
				fmt.Println("do something after connect event")
			}))

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
		URL(RemoteServerUrl),
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

func Test_emptyDecorator(t *testing.T) {
	assert.NoError(t, emptyDecorator(http.Header{}))
}

type QuicSuite struct {
	suite.Suite
	got                          *QuicClient
	mockRedirector               *MockRedirector
	mockDialer                   *MockDialer
	mockConnectEventListeners    *mockevent.MockListeners
	mockDisconnectEventListeners *mockevent.MockListeners
}

func TestQuicSuite(t *testing.T) {
	suite.Run(t, new(QuicSuite))
}

func (suite *QuicSuite) SetupTest() {
	mockConnectEventListeners := &mockevent.MockListeners{}
	mockDisconnectEventListeners := &mockevent.MockListeners{}
	got, err := New(
		Enabled(true),
		URL(RemoteServerUrl),
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
		AddConnectListener(mockConnectEventListeners),
		AddDisconnectListener(mockDisconnectEventListeners),
		Once(true),
	)

	suite.NoError(err)
	suite.NotNil(got)

	mockDialer := NewMockDialer()
	got.qd = mockDialer

	mockRedirector := NewMockRedirector()
	got.rd = mockRedirector
	got.qd = mockDialer

	suite.got = got
	suite.mockRedirector = mockRedirector
	suite.mockDialer = mockDialer
	suite.mockConnectEventListeners = mockConnectEventListeners
	suite.mockDisconnectEventListeners = mockDisconnectEventListeners
}

func (suite *QuicSuite) Test_CancelCtx() {
	mockConn := NewMockConnection()
	mockStr := NewMockStream([]byte("xxxx"))
	mockStr.On("Close").Return(nil)
	mockConn.On("AcceptStream", mock.Anything).Return(mockStr, errors.New("some error"))
	mockConn.On("CloseWithError", mock.Anything, mock.Anything).Return(nil)

	remoteServerUrl, err := url.Parse("RemoteServerUrl")
	suite.NoError(err)
	suite.mockRedirector.On("GetUrl", mock.Anything, mock.Anything).Return(remoteServerUrl, nil)
	suite.mockDialer.On("DialQuic", mock.Anything, mock.Anything).Return(mockConn, nil)

	suite.mockConnectEventListeners.On("OnConnect", mock.Anything).Return()
	suite.mockDisconnectEventListeners.On("OnDisconnect", mock.Anything).Return()

	suite.got.Start()

	time.Sleep(500 * time.Millisecond)
	suite.got.shutdown()
	suite.got.Stop()
	// for suite.got.done == false {
	// 	time.Sleep(10 * time.Millisecond)
	// }
}

func (suite *QuicSuite) TestDialErr() {
	remoteServerUrl, err := url.Parse("RemoteServerUrl")
	suite.NoError(err)
	suite.mockRedirector.On("GetUrl", mock.Anything, mock.Anything).Return(remoteServerUrl, errors.New("some error"))
	suite.mockConnectEventListeners.On("OnConnect", mock.Anything)
	suite.got.Start()

	time.Sleep(20 * time.Millisecond)

	// e := events.Parent.Calls[0].Arguments.Get(0).(event.Connect)
	// suite.Equal(int32(1), e.TriesSinceLastConnect)
	suite.Equal(int32(1), suite.got.triesSinceLastConnect.Load())
	//suite.NotNil(e.Err)
	suite.mockConnectEventListeners.AssertCalled(suite.T(), "OnConnect", mock.Anything)
}

func (suite *QuicSuite) TestSend() {
	mockConn := NewMockConnection()

	mockStr := NewMockStream([]byte(""))

	mockStr.On("Write", mock.Anything).Return(0, nil)
	mockStr.On("Close").Return(nil)

	mockConn.On("OpenStream").Return(mockStr, nil)

	suite.got.conn = mockConn

	msg := wrp.Message{
		Type:        wrp.SimpleEventMessageType,
		Source:      fmt.Sprintf("event:test.com/%s", "client"),
		Destination: "mac:4ca161000109/mock_config",
		PartnerIDs:  []string{"foobar"},
	}
	suite.got.Send(context.Background(), msg)

	mockConn.AssertCalled(suite.T(), "OpenStream")
	mockStr.AssertCalled(suite.T(), "Write", mock.Anything)
	mockStr.AssertCalled(suite.T(), "Close")
	suite.Equal(int32(0), suite.got.triesSinceLastConnect.Load())
}

func (suite *QuicSuite) TestHandleWrp() {
	mockConn := NewMockConnection()

	mockStr := NewMockStream([]byte(""))

	mockStr.On("Write", mock.Anything).Return(0, nil)
	mockStr.On("Close").Return(nil)

	mockConn.On("OpenStream").Return(mockStr, nil)

	suite.got.conn = mockConn

	msg := wrp.Message{
		Type:        wrp.SimpleEventMessageType,
		Source:      fmt.Sprintf("event:test.com/%s", "client"),
		Destination: "mac:4ca161000109/mock_config",
		PartnerIDs:  []string{"foobar"},
	}
	suite.got.HandleWrp(msg)

	mockConn.AssertCalled(suite.T(), "OpenStream")
	mockStr.AssertCalled(suite.T(), "Write", mock.Anything)
	mockStr.AssertCalled(suite.T(), "Close")
	suite.Equal(int32(0), suite.got.triesSinceLastConnect.Load())
}

func (suite *QuicSuite) TestSendError() {
	mockConn := NewMockConnection()

	mockStr := NewMockStream([]byte(""))

	mockStr.On("Close").Return(nil)

	mockConn.On("OpenStream").Return(mockStr, errors.New("some error"))

	suite.got.conn = mockConn

	msg := wrp.Message{
		Type:        wrp.SimpleEventMessageType,
		Source:      fmt.Sprintf("event:test.com/%s", "client"),
		Destination: "mac:4ca161000109/mock_config",
		PartnerIDs:  []string{"foobar"},
	}
	err := suite.got.Send(context.Background(), msg)
	suite.Error(err)

	mockConn.AssertCalled(suite.T(), "OpenStream")
	mockStr.AssertNotCalled(suite.T(), "Write", mock.Anything)
}

func (suite *QuicSuite) TestWriteError() {
	mockConn := NewMockConnection()

	mockStr := NewMockStream([]byte(""))

	mockStr.On("Write", mock.Anything).Return(0, errors.New("some error"))
	mockStr.On("Close").Return(nil)

	mockConn.On("OpenStream").Return(mockStr, nil)

	suite.got.conn = mockConn

	msg := wrp.Message{
		Type:        wrp.SimpleEventMessageType,
		Source:      fmt.Sprintf("event:test.com/%s", "client"),
		Destination: "mac:4ca161000109/mock_config",
		PartnerIDs:  []string{"foobar"},
	}
	err := suite.got.Send(context.Background(), msg)
	suite.Error(err)

	mockConn.AssertCalled(suite.T(), "OpenStream")
	mockStr.AssertCalled(suite.T(), "Write", mock.Anything)
}

func (suite *QuicSuite) TestGetName() {
	suite.Equal("quic", suite.got.Name())
}

func (suite *QuicSuite) TestEnabled() {
	suite.Equal(true, suite.got.IsEnabled())
}

func (suite *QuicSuite) Test_StreamErr() {
	mockConn := NewMockConnection()
	mockStr := NewMockStream([]byte("xxxx"))
	mockStr.On("Close").Return(nil)
	mockConn.On("AcceptStream", mock.Anything).Return(mockStr, errors.New("some error"))
	mockConn.On("CloseWithError", mock.Anything, mock.Anything).Return(nil)

	suite.mockConnectEventListeners.On("OnConnect", mock.Anything).Return()
	suite.mockDisconnectEventListeners.On("OnDisconnect", mock.Anything).Return()

	remoteServerUrl, err := url.Parse("RemoteServerUrl")
	suite.NoError(err)
	suite.mockRedirector.On("GetUrl", mock.Anything, mock.Anything).Return(remoteServerUrl, nil)
	suite.mockDialer.On("DialQuic", mock.Anything, mock.Anything).Return(mockConn, nil)

	suite.got.Start()
	time.Sleep(20 * time.Millisecond)

	suite.mockConnectEventListeners.AssertCalled(suite.T(), "OnConnect", mock.Anything)
	suite.mockDisconnectEventListeners.AssertCalled(suite.T(), "OnDisconnect", mock.Anything)
	mockConn.AssertCalled(suite.T(), "AcceptStream", mock.Anything)
	mockConn.AssertCalled(suite.T(), "CloseWithError", mock.Anything, mock.Anything)
	suite.mockRedirector.AssertCalled(suite.T(), "GetUrl", mock.Anything, mock.Anything)
}

func (suite *QuicSuite) Test_DecodeErr() {
	mockConn := NewMockConnection()
	mockStr := NewMockStream([]byte("xxxx"))
	mockConn.On("AcceptStream", mock.Anything).Return(mockStr, nil)
	mockConn.On("CloseWithError", mock.Anything, mock.Anything).Return(nil)
	mockStr.On("Read", mock.Anything).Return(5, nil)
	mockStr.On("Close").Return(nil)

	suite.mockConnectEventListeners.On("OnConnect", mock.Anything).Return()
	suite.mockDisconnectEventListeners.On("OnDisconnect", mock.Anything).Return()

	remoteServerUrl, err := url.Parse("RemoteServerUrl")
	suite.NoError(err)
	suite.mockRedirector.On("GetUrl", mock.Anything, mock.Anything).Return(remoteServerUrl, nil)
	suite.mockDialer.On("DialQuic", mock.Anything, mock.Anything).Return(mockConn, nil)

	suite.got.Start()
	time.Sleep(10 * time.Millisecond)

	suite.mockConnectEventListeners.AssertCalled(suite.T(), "OnConnect", mock.Anything)
	mockConn.AssertCalled(suite.T(), "AcceptStream", mock.Anything)
	suite.mockDisconnectEventListeners.AssertCalled(suite.T(), "OnDisconnect", mock.Anything)
	//mockConn.AssertCalled(suite.T(), "CloseWithError", mock.Anything)  - this gets called in the debugger
	suite.mockRedirector.AssertCalled(suite.T(), "GetUrl", mock.Anything, mock.Anything)
}
