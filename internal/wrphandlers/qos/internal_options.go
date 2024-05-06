// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"fmt"
)

func validateQueueConstraints() Option {
	return optionFunc(
		func(h *Handler) error {
			if h.maxMessageBytes > h.maxQueueSize {
				return fmt.Errorf("%w: MaxMessageBytes > MaxQueueSize", ErrMisconfiguredQOS)
			}

			return nil
		})
}
