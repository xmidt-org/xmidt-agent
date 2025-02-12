// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package pubsub

import (
	"fmt"
	"time"

	"github.com/xmidt-org/wrp-go/v4"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

type optionFunc func(*PubSub) error

var _ Option = optionFunc(nil)

func (f optionFunc) apply(ps *PubSub) error {
	return f(ps)
}

// Normify is an option that sets the desired normalization options used to
// normalize/validate the wrp message.
//
// As an example, if you want all messages to contain metadata about something
// like the network interface used, this is the place to define it.
func Normify(opts ...wrp.NormifierOption) Option {
	return optionFunc(func(ps *PubSub) error {
		ps.desiredOpts = append(ps.desiredOpts, opts...)
		return nil
	})
}

// WithEgressHandler is an option that adds a handler for egress messages.
// If the optional cancel parameter is provided, it will be set to a function
// that can be used to cancel the subscription.
func WithEgressHandler(handler wrpkit.Handler, cancel ...*CancelFunc) Option {
	return optionFunc(func(ps *PubSub) error {
		c, err := ps.SubscribeEgress(handler)
		if err != nil {
			return err
		}
		if len(cancel) > 0 && cancel[0] != nil {
			*cancel[0] = c
		}

		return nil
	})
}

// WithServiceHandler is an option that adds a handler for service messages.
// If the optional cancel parameter is provided, it will be set to a function
// that can be used to cancel the subscription.
func WithServiceHandler(service string, handler wrpkit.Handler, cancel ...*CancelFunc) Option {
	return optionFunc(func(ps *PubSub) error {
		c, err := ps.SubscribeService(service, handler)
		if err != nil {
			return err
		}
		if len(cancel) > 0 && cancel[0] != nil {
			*cancel[0] = c
		}

		return nil
	})
}

// WithEventHandler is an option that adds a handler for event messages.
// If the optional cancel parameter is provided, it will be set to a function
// that can be used to cancel the subscription.
func WithEventHandler(event string, handler wrpkit.Handler, cancel ...*CancelFunc) Option {
	return optionFunc(func(ps *PubSub) error {
		c, err := ps.SubscribeEvent(event, handler)
		if err != nil {
			return err
		}
		if len(cancel) > 0 && cancel[0] != nil {
			*cancel[0] = c
		}

		return nil
	})
}

// WithPublishTimeout is an option that sets the timeout for publishing a message.
// If the timeout is exceeded, the publish will fail.
func WithPublishTimeout(timeout time.Duration) Option {
	return optionFunc(func(ps *PubSub) error {
		if timeout < 0 {
			return fmt.Errorf("%w: timeout must be zero or larger", ErrInvalidInput)
		}
		ps.publishTimeout = timeout
		return nil
	})
}
