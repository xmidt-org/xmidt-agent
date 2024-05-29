// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"fmt"
)

const (
	DefaultMaxQueueBytes   = 1 * 1024 * 1024 // 1MB max/queue
	DefaultMaxMessageBytes = 256 * 1024      // 256 KB
)

// MaxQueueBytes is the allowable max size of the qos' priority queue, based on the sum of all queued wrp message's payload.
// Note, the default zero behavior is a queue with a 1MB size constraint.
func MaxQueueBytes(s int64) Option {
	return optionFunc(
		func(h *Handler) error {
			if s < 0 {
				return fmt.Errorf("%w: negative MaxQueueBytes", ErrMisconfiguredQOS)
			} else if s == 0 {
				s = DefaultMaxQueueBytes
			}

			h.maxQueueBytes = s

			return nil
		})
}

// MaxMessageBytes is the largest allowable wrp message payload.
// Note, the default zero behavior is a 256KB payload size constraint.
func MaxMessageBytes(s int) Option {
	return optionFunc(
		func(h *Handler) error {
			if s < 0 {
				return fmt.Errorf("%w: negative MaxMessageBytes", ErrMisconfiguredQOS)
			} else if s == 0 {
				s = DefaultMaxMessageBytes
			}

			h.maxMessageBytes = s

			return nil
		})
}

// PrioritizeOldest determines whether to prioritize the oldest message during a QualityOfService tie breaker,
// with the default being to prioritize the newest messages.
func PrioritizeOldest(b bool) Option {
	return optionFunc(
		func(h *Handler) error {
			h.prioritizeOldest = b

			return nil
		})
}
