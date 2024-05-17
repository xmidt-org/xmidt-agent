// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package pubsub

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/xmidt-org/eventor"
	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

var (
	ErrInvalidInput = fmt.Errorf("invalid input")
	ErrTimeout      = fmt.Errorf("timeout")
)

// CancelFunc removes the associated listener with and cancels any future events
// sent to that listener.
//
// A CancelFunc is idempotent: after the first invocation, calling this closure
// will have no effect.
type CancelFunc func()

// PubSub is a struct representing a publish-subscribe system focusing on wrp
// messages.
type PubSub struct {
	lock           sync.RWMutex
	self           wrp.DeviceID
	required       *wrp.Normifier
	desiredOpts    []wrp.NormifierOption
	desired        *wrp.Normifier
	routes         map[string]*eventor.Eventor[wrpkit.Handler]
	publishTimeout time.Duration
}

var _ wrpkit.Handler = (*PubSub)(nil)

// Option is the interface implemented by types that can be used to
// configure the credentials.
type Option interface {
	apply(*PubSub) error
}

// New creates a new instance of the PubSub struct.  The self parameter is the
// device id of the device that is creating the PubSub instance.  During
// publishing, messages will be sent to the appropriate listeners based on the
// service in the message and the device id of the PubSub instance.
func New(self wrp.DeviceID, opts ...Option) (*PubSub, error) {
	if self == "" {
		return nil, fmt.Errorf("%w: self may not be empty", ErrInvalidInput)
	}

	ps := PubSub{
		routes: make(map[string]*eventor.Eventor[wrpkit.Handler]),
		self:   self,
		required: wrp.NewNormifier(
			// Only the absolutely required normalizers are included here.
			wrp.ValidateDestination(),
			wrp.ValidateSource(),
			wrp.ReplaceAnySelfLocator(string(self)),
			wrp.ClampQualityOfService(),
		),
	}

	for _, opt := range opts {
		if opt != nil {
			if err := opt.apply(&ps); err != nil {
				return nil, err
			}
		}
	}

	ps.desired = wrp.NewNormifier(ps.desiredOpts...)

	return &ps, nil
}

// SubscribeEgress subscribes to the egress route.  The listener will be called
// when a message targets something other than this device.  The returned
// CancelFunc may be called to remove the listener and cancel any future events
// sent to that listener.
func (ps *PubSub) SubscribeEgress(h wrpkit.Handler) (CancelFunc, error) {
	return ps.subscribe(egressRoute(), h)
}

// SubscribeService subscribes to the specified service.  The listener will be
// called when a message matches the service.  A service value of '*' may be
// used to match any service.  The returned CancelFunc may be called to remove
// the listener and cancel any future events sent to that listener.
func (ps *PubSub) SubscribeService(service string, h wrpkit.Handler) (CancelFunc, error) {
	if err := validateString(service, "service"); err != nil {
		return nil, err
	}

	return ps.subscribe(serviceRoute(service), h)
}

// SubscribeEvent subscribes to the specified event.  The listener will be called
// when a message matches the event.  An event value of '*' may be used to match
// any event.  The returned CancelFunc may be called to remove the listener and
// cancel any future events sent to that listener.
func (ps *PubSub) SubscribeEvent(event string, h wrpkit.Handler) (CancelFunc, error) {
	if err := validateString(event, "event"); err != nil {
		return nil, err
	}

	return ps.subscribe(eventRoute(event), h)
}

func validateString(s, typ string) error {
	if s == "" {
		return fmt.Errorf("%w: %s may not be empty", ErrInvalidInput, typ)
	}

	disallowed := "/"
	if strings.ContainsAny(s, disallowed) {
		return fmt.Errorf("%w: %s may not contain any of the following: '%s'", ErrInvalidInput, typ, disallowed)
	}

	return nil
}

func (ps *PubSub) subscribe(route string, h wrpkit.Handler) (CancelFunc, error) {
	if h == nil {
		return nil, fmt.Errorf("%w: handler may not be nil", ErrInvalidInput)
	}

	ps.lock.Lock()
	defer ps.lock.Unlock()

	if _, found := ps.routes[route]; !found {
		ps.routes[route] = new(eventor.Eventor[wrpkit.Handler])
	}

	return CancelFunc(ps.routes[route].Add(h)), nil
}

// HandleWrp publishes a wrp message to the appropriate listeners and returns
// if there was at least one handler that accepted the message.  The error
// wrpkit.ErrNotHandled is returned if no listeners were found for the message.
func (ps *PubSub) HandleWrp(msg wrp.Message) error {
	normalized, dest, err := ps.normalize(&msg)
	if err != nil {
		return err
	}

	// Unless the destination is this device, the message will be sent to the
	// egress route.  If the destination is this device, the message will be sent
	// to the service route.
	routes := []string{egressRoute()}
	switch {
	case dest.ID == ps.self:
		routes = []string{
			serviceRoute(dest.Service),
			serviceRoute("*"),
		}
	case dest.Scheme == wrp.SchemeEvent:
		routes = []string{
			eventRoute(dest.Authority),
			eventRoute("*"),
			egressRoute(),
		}
	}

	ps.lock.RLock()
	defer ps.lock.RUnlock()

	wg := sync.WaitGroup{}
	stop := make(chan struct{})
	handled := make(chan struct{}, 1)
	ctx, cancel := context.WithTimeout(context.Background(), ps.publishTimeout)
	defer cancel()

	for _, route := range routes {
		if _, found := ps.routes[route]; found {
			ps.routes[route].Visit(func(h wrpkit.Handler) {
				// By making this a go routine, we can avoid deadlocks if the handler
				// tries to subscribe to the same service.  It also avoids blocking the
				// caller if the handler takes a long time to process the message.
				if h != nil {
					wg.Add(1)
					go func() {
						defer wg.Done()

						err := h.HandleWrp(*normalized)
						if errors.Is(err, wrpkit.ErrNotHandled) {
							return
						}

						// Signal that the message was handled, or stop
						// trying to send the message if the stop channel
						// is closed.
						select {
						case handled <- struct{}{}:
						case <-stop:
						}
					}()
				}
			})
		}
	}

	// Make waiting operate on a channel so that it can be interrupted if the
	// message is handled, or a timeout is reached.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-handled: // No more responses are needed.
		err = nil
	case <-done: // All handlers have finished.
		err = wrpkit.ErrNotHandled
	case <-ctx.Done(): // The timeout has been reached.
		err = ErrTimeout
	}
	close(stop)

	return err
}

func (ps *PubSub) normalize(msg *wrp.Message) (*wrp.Message, wrp.Locator, error) {
	if err := ps.required.Normify(msg); err != nil {
		return nil, wrp.Locator{}, err
	}

	// These have already been validated by the required normifier.
	dst, _ := wrp.ParseLocator(msg.Destination)
	src, _ := wrp.ParseLocator(msg.Source)

	if src.ID == ps.self {
		// Apply the additional normalization for messages that originated from this
		// device.
		if err := ps.desired.Normify(msg); err != nil {
			return nil, wrp.Locator{}, err
		}
	}

	return msg, dst, nil
}

func serviceRoute(service string) string {
	return "service:" + service
}

func egressRoute() string {
	return "egress:*"
}

func eventRoute(event string) string {
	return "event:" + event
}
