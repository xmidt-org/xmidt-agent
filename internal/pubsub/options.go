// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package pubsub

import (
	"github.com/xmidt-org/wrp-go/v3"
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
func WithEgressHandler(handler Handler, cancel ...*CancelFunc) Option {
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
func WithServiceHandler(service string, handler Handler, cancel ...*CancelFunc) Option {
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
func WithEventHandler(event string, handler Handler, cancel ...*CancelFunc) Option {
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
