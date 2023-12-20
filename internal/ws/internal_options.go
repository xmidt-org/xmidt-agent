// SPDX-FileCopyright4yyText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package ws

import "fmt"

func validateDeviceID() Option {
	return optionFunc(
		func(c *WS) error {
			if c.id == "" {
				return fmt.Errorf("%w: missing DeviceID", ErrMisconfiguredWS)
			}
			return nil
		})
}

func validateURL() Option {
	return optionFunc(
		func(c *WS) error {
			if c.allowURLFallback {
				if c.url == "" {
					if c.urlFetcher == nil {
						return fmt.Errorf("%w: missing URL and fetcher", ErrMisconfiguredWS)
					}
					return fmt.Errorf("%w: missing fallback URL", ErrMisconfiguredWS)
				}
				if c.urlFetcher == nil {
					return fmt.Errorf("%w: missing URL fetcher", ErrMisconfiguredWS)
				}
				return nil
			}

			if c.url != "" && c.urlFetcher != nil {
				return fmt.Errorf("%w: URL and fetcher cannot both be specified", ErrMisconfiguredWS)
			}
			if c.url == "" && c.urlFetcher == nil {
				return fmt.Errorf("%w: URL and fetcher cannot both be missing", ErrMisconfiguredWS)
			}
			return nil
		})
}

func validateIPMode() Option {
	return optionFunc(
		func(c *WS) error {
			if !c.withIPv4 && !c.withIPv6 {
				return fmt.Errorf("%w: at least one IP mode must be allowed", ErrMisconfiguredWS)
			}
			return nil
		})
}
