// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"fmt"

	"github.com/xmidt-org/wrp-go/v3"
)

// AddPriorityQueue sets the max size for the qos queue
func AddPriorityQueue(maxQueueSize, maxQueueDepth int) Option {
	return optionFunc(
		func(h *Handler) error {
			if maxQueueSize < 0 {
				return fmt.Errorf("%w: negative MaxQueueSize", ErrMisconfiguredWS)
			} else if maxQueueDepth < 0 {
				return fmt.Errorf("%w: negative maxQueueDepth", ErrMisconfiguredWS)
			}

			h.queue = PriorityQueue{
				queue:        make([]wrp.Message, 0, maxQueueDepth),
				maxQueueSize: maxQueueSize,
			}

			return nil
		})
}
