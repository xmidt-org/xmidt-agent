// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"fmt"
)

// MaxHeapSize is the allowable max size of the qos' priority queue, based on the sum of all queued wrp message's payload
func MaxHeapSize(s int) Option {
	return optionFunc(
		func(h *Handler) error {
			if s < 0 {
				return fmt.Errorf("%w: negative MaxHeapSize", ErrMisconfiguredQOS)
			}

			h.maxHeapSize = s

			return nil
		})
}
