// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"fmt"

	"github.com/xmidt-org/wrp-go/v3"
)

// AddPriorityQueue configures and adds a qos priority queue
func AddPriorityQueue(maxQueueSize, maxQueueDepth int) Option {
	return optionFunc(
		func(h *Handler) error {
			if maxQueueSize < 0 {
				return fmt.Errorf("%w: negative MaxQueueSize", ErrMisconfiguredQOS)
			} else if maxQueueDepth < 0 {
				return fmt.Errorf("%w: negative maxQueueDepth", ErrMisconfiguredQOS)
			}

			h.queue = PriorityQueue{
				queue:        make([]wrp.Message, 0, maxQueueDepth),
				maxQueueSize: maxQueueSize,
			}

			return nil
		})
}
