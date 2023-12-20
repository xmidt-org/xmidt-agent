// SPDX-FileCopyright4yyText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package ws

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/ws/event"
)

// DeviceID sets the device ID for the WS connection.
func DeviceID(id wrp.DeviceID) Option {
	return optionFunc(
		func(ws *WS) error {
			ws.id = id
			ws.additionalHeaders.Set("X-Webpa-Device-Name", id.ID())
			return nil
		})
}

// URL sets the URL for the WS connection.
func URL(url string) Option {
	return optionFunc(
		func(ws *WS) error {
			ws.url = url
			return nil
		})
}

// URLFetcher sets the URLFetcher for the WS connection.
func URLFetcher(f func(context.Context) (string, error)) Option {
	return optionFunc(
		func(ws *WS) error {
			ws.urlFetcher = f
			return nil
		})
}

// URLFetchingTimeout sets the URLFetchingTimeout for the WS connection.
// If this is not set, the default is 30 seconds.
func URLFetchingTimeout(d time.Duration) Option {
	return optionFunc(
		func(ws *WS) error {
			if d < 0 {
				return fmt.Errorf("%w: negative URLFetchingTimeout", ErrMisconfiguredWS)
			}
			ws.urlFetchingTimeout = d
			return nil
		})
}

// CredentialsDecorator provides the credentials decorator for the WS connection.
func CredentialsDecorator(f func(http.Header) error) Option {
	return optionFunc(
		func(ws *WS) error {
			ws.credDecorator = f
			return nil
		})
}

// PingInterval sets the time expected between PINGs for the WS connection.
// If this is not set, the default is 30 seconds.
func PingInterval(d time.Duration) Option {
	return optionFunc(
		func(ws *WS) error {
			ws.pingInterval = d
			return nil
		})
}

// PingTimeout sets the maximum time allowed between PINGs for the WS connection
// before the connection is closed.  If this is not set, the default is 90 seconds.
func PingTimeout(d time.Duration) Option {
	return optionFunc(
		func(ws *WS) error {
			ws.pingTimeout = d
			return nil
		})
}

// KeepAliveInterval sets the keep alive interval for the WS connection.
// If this is not set, the default is 30 seconds.
func KeepAliveInterval(d time.Duration) Option {
	return optionFunc(
		func(ws *WS) error {
			ws.keepAliveInterval = d
			return nil
		})
}

// TLSHandshakeTimeout sets the TLS handshake timeout for the WS connection.
// If this is not set, the default is 10 seconds.
func TLSHandshakeTimeout(d time.Duration) Option {
	return optionFunc(
		func(ws *WS) error {
			ws.tlsHandshakeTimeout = d
			return nil
		})
}

// IdleConnTimeout sets the idle connection timeout for the WS connection.
// If this is not set, the default is 10 seconds.
func IdleConnTimeout(d time.Duration) Option {
	return optionFunc(
		func(ws *WS) error {
			ws.idleConnTimeout = d
			return nil
		})
}

// ExpectContinueTimeout sets the expect continue timeout for the WS connection.
// If this is not set, the default is 1 second.
func ExpectContinueTimeout(d time.Duration) Option {
	return optionFunc(
		func(ws *WS) error {
			ws.expectContinueTimeout = d
			return nil
		})
}

// WithIPv4 sets whether or not to allow IPv4 for the WS connection.  If this
// is not set, the default is true.
func WithIPv4(with ...bool) Option {
	if len(with) == 0 {
		with = []bool{true}
	}
	return optionFunc(
		func(ws *WS) error {
			ws.withIPv4 = with[0]
			return nil
		})
}

// WithIPv6 sets whether or not to allow IPv6 for the WS connection.  If this
// is not set, the default is true.
func WithIPv6(with ...bool) Option {
	if len(with) == 0 {
		with = []bool{true}
	}
	return optionFunc(
		func(ws *WS) error {
			ws.withIPv6 = with[0]
			return nil
		})
}

// ConnectTimeout sets the timeout for the WS connection.  If this is not set,
// the default is 30 seconds.
func ConnectTimeout(d time.Duration) Option {
	return optionFunc(
		func(ws *WS) error {
			if d < 0 {
				return fmt.Errorf("%w: negative ConnectTimeout", ErrMisconfiguredWS)
			}
			ws.connectTimeout = d
			return nil
		})
}

// AdditionalHeaders sets the additional headers for the WS connection.
func AdditionalHeaders(headers http.Header) Option {
	return optionFunc(
		func(ws *WS) error {
			for k, values := range headers {
				for _, value := range values {
					ws.additionalHeaders.Add(k, value)
				}
			}
			return nil
		})
}

// AllowURLFallback sets whether or not to allow URL fallback for the WS connection.
// If this is not set, the default is true.
func AllowURLFallback(allow ...bool) Option {
	if len(allow) == 0 {
		allow = []bool{true}
	}
	return optionFunc(
		func(ws *WS) error {
			ws.allowURLFallback = allow[0]
			return nil
		})
}

// AddConnectListener adds a connect listener to the WS connection.
func AddConnectListener(listener event.ConnectListener) Option {
	return optionFunc(
		func(ws *WS) error {
			ws.connectListeners.Add(listener)
			return nil
		})
}

// AddDisconnectListener adds a disconnect listener to the WS connection.
func AddDisconnectListener(listener event.DisconnectListener) Option {
	return optionFunc(
		func(ws *WS) error {
			ws.disconnectListeners.Add(listener)
			return nil
		})
}

// AddHeartbeatListener adds a heartbeat listener to the WS connection.
func AddHeartbeatListener(listener event.HeartbeatListener) Option {
	return optionFunc(
		func(ws *WS) error {
			ws.heartbeatListeners.Add(listener)
			return nil
		})
}

// NowFunc sets the now function for the WS connection.
func NowFunc(f func() time.Time) Option {
	return optionFunc(
		func(ws *WS) error {
			if f == nil {
				return fmt.Errorf("%w: nil NowFunc", ErrMisconfiguredWS)
			}
			ws.nowFunc = f
			return nil
		})
}

// RetryPolicy sets the retry policy for the WS connection.
func RetryPolicy(rp retry.Policy) Option {
	return optionFunc(
		func(ws *WS) error {
			ws.retryPolicy = rp
			return nil
		})
}
