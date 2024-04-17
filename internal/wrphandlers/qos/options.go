// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import "fmt"

// MaxQueueSize sets the max size for the qos queue
func MaxQueueSize(s int) Option {
	return optionFunc(
		func(h *Handler) error {
			if s < 0 {
				return fmt.Errorf("%w: negative MaxQueueSize", ErrMisconfiguredWS)
			}

			h.maxQueueSize = s
			return nil
		})
}
