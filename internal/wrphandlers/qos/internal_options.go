// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"fmt"
)

func validateQueueConstraints() Option {
	return optionFunc(
		func(h *Handler) error {
			if int64(h.maxMessageBytes) > h.maxQueueBytes {
				return fmt.Errorf("%w: MaxMessageBytes > MaxQueueBytes", ErrMisconfiguredQOS)
			}

			return nil
		})
}
