// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package quic

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/event"
)

// DeviceID sets the device ID for the WS connection.
func DeviceID(id wrp.DeviceID) Option {
	return optionFunc(
		func(ws *QuicClient) error {
			if id == "" {
				return fmt.Errorf("%w: empty DeviceID", ErrMisconfiguredQuic)
			}

			ws.id = id
			if ws.additionalHeaders == nil {
				ws.additionalHeaders = http.Header{}
			}

			ws.additionalHeaders.Set("X-Webpa-Device-Name", string(id))
			return nil
		})
}

// URL sets the URL for the quic connection.
func URL(url string) Option {
	return optionFunc(
		func(ws *QuicClient) error {
			if url == "" {
				return fmt.Errorf("%w: empty URL", ErrMisconfiguredQuic)
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
		func(ws *QuicClient) error {
			if f == nil {
				return fmt.Errorf("%w: nil FetchURL", ErrMisconfiguredQuic)
			}

			ws.urlFetcher = f
			return nil
		})
}

// FetchURLTimeout sets the FetchURLTimeout for the WS connection.
// If this is not set, the default is 30 seconds.
func FetchURLTimeout(d time.Duration) Option {
	return optionFunc(
		func(ws *QuicClient) error {
			if d < 0 {
				return fmt.Errorf("%w: negative FetchURLTimeout", ErrMisconfiguredQuic)
			}

			ws.urlFetchingTimeout = d
			return nil
		})
}

// CredentialsDecorator provides the credentials decorator for the WS connection.
func CredentialsDecorator(f func(http.Header) error) Option {
	return optionFunc(
		func(ws *QuicClient) error {
			if f == nil {
				return fmt.Errorf("%w: nil CredentialsDecorator", ErrMisconfiguredQuic)
			}

			ws.credDecorator = f
			return nil
		})
}

func ConveyDecorator(f func(http.Header) error) Option {
	return optionFunc(
		func(ws *QuicClient) error {
			if f == nil {
				return fmt.Errorf("%w: nil ConveyDecorator", ErrMisconfiguredQuic)
			}

			ws.conveyDecorator = f
			return nil
		})
}

// KeepAliveInterval sets the keep alive interval for the WS connection.
// If this is not set, the default is 30 seconds.
func KeepAliveInterval(d time.Duration) Option {
	return optionFunc(
		func(ws *QuicClient) error {
			if d < 0 {
				return fmt.Errorf("%w: negative KeepAliveInterval", ErrMisconfiguredQuic)
			}

			ws.keepAliveInterval = d
			return nil
		})
}

// SendTimeout sets the send timeout for the WS connection.
func SendTimeout(d time.Duration) Option {
	return optionFunc(
		func(ws *QuicClient) error {
			if d < 0 {
				return fmt.Errorf("%w: negative SendTimeout", ErrMisconfiguredQuic)
			}

			ws.sendTimeout = d
			return nil
		})
}

// HTTPClient is the configuration for the HTTP3 client used for connection attempts.
func HTTP3Client(c *Http3ClientConfig) Option {
	return optionFunc(
		func(qc *QuicClient) error {

			qc.http3ClientConfig = c

			return nil
		})
}

// AdditionalHeaders sets the additional headers for the WS connection.
func AdditionalHeaders(headers http.Header) Option {
	return optionFunc(
		func(qc *QuicClient) error {
			for k, values := range headers {
				for _, value := range values {
					qc.additionalHeaders.Add(k, value)
				}
			}

			return nil
		})
}

// Once sets whether or not to only attempt to connect once.
func Once(once ...bool) Option {
	once = append(once, true)
	return optionFunc(
		func(qc *QuicClient) error {
			qc.once = once[0]
			return nil
		})
}

func Enabled(enabled bool) Option {
	return optionFunc(
		func(qc *QuicClient) error {
			qc.enabled = enabled
			return nil
		})
}

// NowFunc sets the now function for the WS connection.
func NowFunc(f func() time.Time) Option {
	return optionFunc(
		func(ws *QuicClient) error {
			if f == nil {
				return fmt.Errorf("%w: nil NowFunc", ErrMisconfiguredQuic)
			}

			ws.nowFunc = f
			return nil
		})
}

// RetryPolicy sets the retry policy factory used for delaying between retry
// attempts for reconnection.
func RetryPolicy(pf retry.PolicyFactory) Option {
	return optionFunc(
		func(ws *QuicClient) error {
			if pf == nil {
				return fmt.Errorf("%w: nil RetryPolicy", ErrMisconfiguredQuic)
			}

			ws.retryPolicyFactory = pf
			return nil
		})
}

// AddMessageListener adds a message listener to the WS connection.
// The listener will be called for every message received from the WS.
func AddMessageListener(listener event.MsgListener, cancel ...*event.CancelFunc) Option {
	return optionFunc(
		func(ws *QuicClient) error {
			var ignored event.CancelFunc
			cancel = append(cancel, &ignored)
			*cancel[0] = event.CancelFunc(ws.msgListeners.Add(listener))
			return nil
		})
}

// AddConnectListener adds a connect listener to the WS connection.
func AddConnectListener(listener event.ConnectListener, cancel ...*event.CancelFunc) Option {
	return optionFunc(
		func(ws *QuicClient) error {
			var ignored event.CancelFunc
			cancel = append(cancel, &ignored)
			*cancel[0] = event.CancelFunc(ws.connectListeners.Add(listener))
			return nil
		})
}

// AddDisconnectListener adds a disconnect listener to the WS connection.
func AddDisconnectListener(listener event.DisconnectListener, cancel ...*event.CancelFunc) Option {
	return optionFunc(
		func(ws *QuicClient) error {
			var ignored event.CancelFunc
			cancel = append(cancel, &ignored)
			*cancel[0] = event.CancelFunc(ws.disconnectListeners.Add(listener))
			return nil
		})
}
