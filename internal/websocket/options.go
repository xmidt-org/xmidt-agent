// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/xmidt-org/arrange/arrangehttp"
	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/event"
)

// DeviceID sets the device ID for the WS connection.
func DeviceID(id wrp.DeviceID) Option {
	return optionFunc(
		func(ws *Websocket) error {
			if id == "" {
				return fmt.Errorf("%w: empty DeviceID", ErrMisconfiguredWS)
			}

			ws.id = id
			if ws.additionalHeaders == nil {
				ws.additionalHeaders = http.Header{}
			}

			ws.additionalHeaders.Set("X-Webpa-Device-Name", string(id))
			return nil
		})
}

// URL sets the URL for the WS connection.
func URL(url string) Option {
	return optionFunc(
		func(ws *Websocket) error {
			if url == "" {
				return fmt.Errorf("%w: empty URL", ErrMisconfiguredWS)
			}

			ws.urlFetcher = func(context.Context) (string, error) {
				return url, nil
			}
			return nil
		})
}

// FetchURL sets the FetchURL for the WS connection.
func FetchURL(f func(context.Context) (string, error)) Option {
	return optionFunc(
		func(ws *Websocket) error {
			if f == nil {
				return fmt.Errorf("%w: nil FetchURL", ErrMisconfiguredWS)
			}

			ws.urlFetcher = f
			return nil
		})
}

// FetchURLTimeout sets the FetchURLTimeout for the WS connection.
// If this is not set, the default is 30 seconds.
func FetchURLTimeout(d time.Duration) Option {
	return optionFunc(
		func(ws *Websocket) error {
			if d < 0 {
				return fmt.Errorf("%w: negative FetchURLTimeout", ErrMisconfiguredWS)
			}

			ws.urlFetchingTimeout = d
			return nil
		})
}

// CredentialsDecorator provides the credentials decorator for the WS connection.
func CredentialsDecorator(f func(http.Header) error) Option {
	return optionFunc(
		func(ws *Websocket) error {
			if f == nil {
				return fmt.Errorf("%w: nil CredentialsDecorator", ErrMisconfiguredWS)
			}

			ws.credDecorator = f
			return nil
		})
}

func ConveyDecorator(f func(http.Header) error) Option {
	return optionFunc(
		func(ws *Websocket) error {
			if f == nil {
				return fmt.Errorf("%w: nil ConveyDecorator", ErrMisconfiguredWS)
			}

			ws.conveyDecorator = f
			return nil
		})
}

// InactivityTimeout sets inactivity timeout for the WS connection.
func InactivityTimeout(d time.Duration) Option {
	return optionFunc(
		func(ws *Websocket) error {
			if d < 0 {
				return fmt.Errorf("%w: negative InactivityTimeout", ErrMisconfiguredWS)
			}

			ws.inactivityTimeout = d
			return nil
		})
}

// PingWriteTimeout sets the maximum time allowed between PINGs for the WS connection
// before the connection is closed.  If this is not set, the default is 90 seconds.
func PingWriteTimeout(d time.Duration) Option {
	return optionFunc(
		func(ws *Websocket) error {
			if d < 0 {
				return fmt.Errorf("%w: negative PingWriteTimeout", ErrMisconfiguredWS)
			}

			ws.pingWriteTimeout = d
			return nil
		})
}

// KeepAliveInterval sets the keep alive interval for the WS connection.
// If this is not set, the default is 30 seconds.
func KeepAliveInterval(d time.Duration) Option {
	return optionFunc(
		func(ws *Websocket) error {
			if d < 0 {
				return fmt.Errorf("%w: negative KeepAliveInterval", ErrMisconfiguredWS)
			}

			ws.keepAliveInterval = d
			return nil
		})
}

// WithIPv4 sets whether or not to allow IPv4 for the WS connection.  If this
// is not set, the default is true.
func WithIPv4(with ...bool) Option {
	with = append(with, true)
	return optionFunc(
		func(ws *Websocket) error {
			ws.withIPv4 = with[0]
			return nil
		})
}

// WithIPv6 sets whether or not to allow IPv6 for the WS connection.  If this
// is not set, the default is true.
func WithIPv6(with ...bool) Option {
	with = append(with, true)
	return optionFunc(
		func(ws *Websocket) error {
			ws.withIPv6 = with[0]
			return nil
		})
}

// SendTimeout sets the send timeout for the WS connection.
func SendTimeout(d time.Duration) Option {
	return optionFunc(
		func(ws *Websocket) error {
			if d < 0 {
				return fmt.Errorf("%w: negative SendTimeout", ErrMisconfiguredWS)
			}

			ws.sendTimeout = d
			return nil
		})
}

// HTTPClient is the configuration for the HTTP client used for connection attempts.
func HTTPClient(c arrangehttp.ClientConfig) Option {
	return optionFunc(
		func(ws *Websocket) error {
			if _, err := c.NewClient(); err != nil {
				return errors.Join(err, ErrMisconfiguredWS)
			}

			ws.httpClientConfig = c

			return nil
		})
}

// HTTPClientWithForceSets is the configuration for the HTTP client with recommended force sets, used for connection attempts.
func HTTPClientWithForceSets(c arrangehttp.ClientConfig) Option {
	return optionFunc(
		func(ws *Websocket) (err error) {
			// Set the configuration
			if err := HTTPClient(c).apply(ws); err != nil {
				return err
			}

			// Override the following arrangehttp.ClientConfig.Transport feilds
			// Note, arrangehttp.ClientConfig.Transport lacks the http.Transport.Proxy,
			// instead `Proxy` will be set during Websocket.newHTTPClient()
			ws.httpClientConfig.Transport.MaxIdleConns = 1
			ws.httpClientConfig.Transport.MaxIdleConnsPerHost = 1
			ws.httpClientConfig.Transport.MaxConnsPerHost = 1

			return nil
		})
}

// AdditionalHeaders sets the additional headers for the WS connection.
func AdditionalHeaders(headers http.Header) Option {
	return optionFunc(
		func(ws *Websocket) error {
			for k, values := range headers {
				for _, value := range values {
					ws.additionalHeaders.Add(k, value)
				}
			}

			return nil
		})
}

// Once sets whether or not to only attempt to connect once.
func Once(once ...bool) Option {
	once = append(once, true)
	return optionFunc(
		func(ws *Websocket) error {
			ws.once = once[0]
			return nil
		})
}

// NowFunc sets the now function for the WS connection.
func NowFunc(f func() time.Time) Option {
	return optionFunc(
		func(ws *Websocket) error {
			if f == nil {
				return fmt.Errorf("%w: nil NowFunc", ErrMisconfiguredWS)
			}

			ws.nowFunc = f
			return nil
		})
}

// RetryPolicy sets the retry policy factory used for delaying between retry
// attempts for reconnection.
func RetryPolicy(pf retry.PolicyFactory) Option {
	return optionFunc(
		func(ws *Websocket) error {
			if pf == nil {
				return fmt.Errorf("%w: nil RetryPolicy", ErrMisconfiguredWS)
			}

			ws.retryPolicyFactory = pf
			return nil
		})
}

// MaxMessageBytes sets the maximum message size sent or received in bytes.
func MaxMessageBytes(bytes int64) Option {
	return optionFunc(
		func(ws *Websocket) error {
			if bytes < 0 {
				return fmt.Errorf("%w: negative MaxMessageBytes", ErrMisconfiguredWS)
			}

			ws.maxMessageBytes = bytes
			return nil
		})
}

// AddMessageListener adds a message listener to the WS connection.
// The listener will be called for every message received from the WS.
func AddMessageListener(listener event.MsgListener, cancel ...*event.CancelFunc) Option {
	return optionFunc(
		func(ws *Websocket) error {
			var ignored event.CancelFunc
			cancel = append(cancel, &ignored)
			*cancel[0] = event.CancelFunc(ws.msgListeners.Add(listener))
			return nil
		})
}

// AddConnectListener adds a connect listener to the WS connection.
func AddConnectListener(listener event.ConnectListener, cancel ...*event.CancelFunc) Option {
	return optionFunc(
		func(ws *Websocket) error {
			var ignored event.CancelFunc
			cancel = append(cancel, &ignored)
			*cancel[0] = event.CancelFunc(ws.connectListeners.Add(listener))
			return nil
		})
}

// AddDisconnectListener adds a disconnect listener to the WS connection.
func AddDisconnectListener(listener event.DisconnectListener, cancel ...*event.CancelFunc) Option {
	return optionFunc(
		func(ws *Websocket) error {
			var ignored event.CancelFunc
			cancel = append(cancel, &ignored)
			*cancel[0] = event.CancelFunc(ws.disconnectListeners.Add(listener))
			return nil
		})
}

// AddHeartbeatListener adds a heartbeat listener to the WS connection.
func AddHeartbeatListener(listener event.HeartbeatListener, cancel ...*event.CancelFunc) Option {
	return optionFunc(
		func(ws *Websocket) error {
			var ignored event.CancelFunc
			cancel = append(cancel, &ignored)
			*cancel[0] = event.CancelFunc(ws.heartbeatListeners.Add(listener))
			return nil
		})
}
