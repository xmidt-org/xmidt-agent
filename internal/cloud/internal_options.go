// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"fmt"
)

func validateQuic() Option {
	return optionFunc(
		func(p *Proxy) error {
			if p.qc == nil {
				return fmt.Errorf("%w: missing quic", ErrMisconfiguredCloud)
			}
			return nil
		})
}

func validateWebsocket() Option {
	return optionFunc(
		func(p *Proxy) error {
			if p.ws == nil {
				return fmt.Errorf("%w: missing websocket", ErrMisconfiguredCloud)
			}
			return nil
		})
}
