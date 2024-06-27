// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"errors"
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

func validatePriority() Option {
	return optionFunc(
		func(h *Handler) error {
			if h.priority <= UnknownType || h.priority >= lastType {
				return errors.Join(fmt.Errorf("%w: %s", ErrPriorityTypeInvalid, h.priority), ErrMisconfiguredQOS)
			}

			return nil
		})
}

func validateTieBreaker() Option {
	return optionFunc(
		func(h *Handler) error {
			if h.tieBreaker == nil || h.trimTieBreaker == nil {
				return errors.Join(fmt.Errorf("%w: nil tiebreak/trimTieBreaker", ErrPriorityTypeInvalid), ErrMisconfiguredQOS)
			}

			return nil
		})
}
