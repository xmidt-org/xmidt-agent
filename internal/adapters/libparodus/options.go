// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package libparodus

import (
	"fmt"
	"time"
)

func KeepaliveInterval(timeout time.Duration) Option {
	return optionFunc(func(s *Adapter) error {
		s.keepaliveInterval = timeout
		return nil
	})
}

func ReceiveTimeout(timeout time.Duration) Option {
	return optionFunc(func(s *Adapter) error {
		s.recvTimeout = timeout
		return nil
	})
}

func SendTimeout(timeout time.Duration) Option {
	return optionFunc(func(s *Adapter) error {
		s.sendTimeout = timeout
		return nil
	})
}

// -- Only Validators Below ----------------------------------------------------

func validatePubSub() Option {
	return optionFunc(func(s *Adapter) error {
		if s.pubsub == nil {
			return fmt.Errorf("%w: pubsub is required", ErrInvalidInput)
		}

		return nil
	})
}

func validateParodusServiceURL() Option {
	return optionFunc(func(s *Adapter) error {
		if s.parodusServiceURL == "" {
			return fmt.Errorf("%w: service url is required", ErrInvalidInput)
		}

		return nil
	})
}
