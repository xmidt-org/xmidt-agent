// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"fmt"
)

const DefaultMaxQueueSize = 1 * 1024 * 1024 // 1MB max/queue,

// MaxQueueSize is the allowable max size of the qos' priority queue, based on the sum of all queued wrp message's payload
// Note, the default zero behavior is a queue with unbound len/cap
func MaxQueueSize(s int) Option {
	return optionFunc(
		func(h *Handler) error {
			if s < 0 {
				return fmt.Errorf("%w: negative MaxQueueSize", ErrMisconfiguredQOS)
			} else if s == 0 {
				s = DefaultMaxQueueSize
			}

			h.maxQueueSize = s

			return nil
		})
}
