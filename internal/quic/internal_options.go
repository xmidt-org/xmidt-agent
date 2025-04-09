// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package quic

import (
	"fmt"
)

func validateDeviceID() Option {
	return optionFunc(
		func(c *QuicClient) error {
			if c.id == "" {
				return fmt.Errorf("%w: missing DeviceID", ErrMisconfiguredQuic)
			}
			return nil
		})
}

func validateURL() Option {
	return optionFunc(
		func(c *QuicClient) error {
			if c.urlFetcher == nil {
				return fmt.Errorf("%w: missing URL fetcher", ErrMisconfiguredQuic)
			}
			return nil
		})
}

func validateFetchURL() Option {
	return optionFunc(
		func(ws *QuicClient) error {
			if ws.urlFetcher == nil {
				return fmt.Errorf("%w: nil FetchURL", ErrMisconfiguredQuic)
			}
			return nil
		})
}

func validateCredentialsDecorator() Option {
	return optionFunc(
		func(ws *QuicClient) error {
			if ws.credDecorator == nil {
				return fmt.Errorf("%w: nil CredentialsDecorator", ErrMisconfiguredQuic)
			}
			return nil
		})
}

func validateConveyDecorator() Option {
	return optionFunc(
		func(ws *QuicClient) error {
			if ws.conveyDecorator == nil {
				return fmt.Errorf("%w: nil ConveyDecorator", ErrMisconfiguredQuic)
			}
			return nil
		})
}

func validateNowFunc() Option {
	return optionFunc(
		func(ws *QuicClient) error {
			if ws.nowFunc == nil {
				return fmt.Errorf("%w: nil NowFunc", ErrMisconfiguredQuic)
			}
			return nil
		})
}

func validRetryPolicy() Option {
	return optionFunc(
		func(ws *QuicClient) error {
			if ws.retryPolicyFactory == nil {
				return fmt.Errorf("%w: nil RetryPolicy", ErrMisconfiguredQuic)
			}
			return nil
		})
}
