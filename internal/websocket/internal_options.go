// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"fmt"
)

func validateDeviceID() Option {
	return optionFunc(
		func(c *Websocket) error {
			if c.id == "" {
				return fmt.Errorf("%w: missing DeviceID", ErrMisconfiguredWS)
			}
			return nil
		})
}

func validateURL() Option {
	return optionFunc(
		func(c *Websocket) error {
			if c.urlFetcher == nil {
				return fmt.Errorf("%w: missing URL fetcher", ErrMisconfiguredWS)
			}
			return nil
		})
}

func validateIPMode() Option {
	return optionFunc(
		func(c *Websocket) error {
			if !c.withIPv4 && !c.withIPv6 {
				return fmt.Errorf("%w: at least one IP mode must be allowed", ErrMisconfiguredWS)
			}
			return nil
		})
}

func validateFetchURL() Option {
	return optionFunc(
		func(ws *Websocket) error {
			if ws.urlFetcher == nil {
				return fmt.Errorf("%w: nil FetchURL", ErrMisconfiguredWS)
			}
			return nil
		})
}

func validateCredentialsDecorator() Option {
	return optionFunc(
		func(ws *Websocket) error {
			if ws.credDecorator == nil {
				return fmt.Errorf("%w: nil CredentialsDecorator", ErrMisconfiguredWS)
			}
			return nil
		})
}

func validateConveyDecorator() Option {
	return optionFunc(
		func(ws *Websocket) error {
			if ws.conveyDecorator == nil {
				return fmt.Errorf("%w: nil ConveyDecorator", ErrMisconfiguredWS)
			}
			return nil
		})
}

func validateNowFunc() Option {
	return optionFunc(
		func(ws *Websocket) error {
			if ws.nowFunc == nil {
				return fmt.Errorf("%w: nil NowFunc", ErrMisconfiguredWS)
			}
			return nil
		})
}

func validRetryPolicy() Option {
	return optionFunc(
		func(ws *Websocket) error {
			if ws.retryPolicyFactory == nil {
				return fmt.Errorf("%w: nil RetryPolicy", ErrMisconfiguredWS)
			}
			return nil
		})
}
