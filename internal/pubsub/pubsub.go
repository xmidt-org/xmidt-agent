// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package pubsub

import (
	"fmt"
	"strings"
	"sync"

	"github.com/xmidt-org/eventor"
	"github.com/xmidt-org/wrp-go/v3"
)

var (
	ErrInvalidInput = fmt.Errorf("invalid input")
)

// CancelFunc removes the associated listener with and cancels any future events
// sent to that listener.
//
// A CancelFunc is idempotent: after the first invocation, calling this closure
// will have no effect.
type CancelFunc func()

// Handler is a function that is called whenever a message is received that
// matches the service associated with the handler.
// listening handler.
type Handler interface {
	// HandleWrp is called whenever a message is received that matches the
	// service associated with the handler.
	HandleWrp(wrp.Message)
}

// HandlerFunc is an adapter to allow the use of ordinary functions as handlers.
type HandlerFunc func(wrp.Message)

func (f HandlerFunc) HandleWrp(msg wrp.Message) {
	f(msg)
}

var _ Handler = HandlerFunc(nil)

// PubSub is a struct representing a publish-subscribe system focusing on wrp
// messages.
type PubSub struct {
	lock        sync.RWMutex
	self        wrp.DeviceID
	required    *wrp.Normifier
	desiredOpts []wrp.NormifierOption
	desired     *wrp.Normifier
	routes      map[string]*eventor.Eventor[Handler]
}

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
		routes: make(map[string]*eventor.Eventor[Handler]),
		self:   self,
		required: wrp.NewNormifier(
			// Only the absolutely required normalizers are included here.
			wrp.ValidateDestination(),
			wrp.ValidateSource(),
			wrp.ReplaceAnySelfLocator(string(self)),
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
func (ps *PubSub) SubscribeEgress(h Handler) (CancelFunc, error) {
	return ps.subscribe(egressRoute(), h)
}

// SubscribeService subscribes to the specified service.  The listener will be
// called when a message matches the service.  A service value of '*' may be
// used to match any service.  The returned CancelFunc may be called to remove
// the listener and cancel any future events sent to that listener.
func (ps *PubSub) SubscribeService(service string, h Handler) (CancelFunc, error) {
	if err := validateString(service, "service"); err != nil {
		return nil, err
	}

	return ps.subscribe(serviceRoute(service), h)
}

// SubscribeEvent subscribes to the specified event.  The listener will be called
// when a message matches the event.  An event value of '*' may be used to match
// any event.  The returned CancelFunc may be called to remove the listener and
// cancel any future events sent to that listener.
func (ps *PubSub) SubscribeEvent(event string, h Handler) (CancelFunc, error) {
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

func (ps *PubSub) subscribe(route string, h Handler) (CancelFunc, error) {
	if h == nil {
		return nil, fmt.Errorf("%w: handler may not be nil", ErrInvalidInput)
	}

	ps.lock.Lock()
	defer ps.lock.Unlock()

	if _, found := ps.routes[route]; !found {
		ps.routes[route] = new(eventor.Eventor[Handler])
	}

	return CancelFunc(ps.routes[route].Add(h)), nil
}

// Publish publishes a wrp message to the appropriate listeners.
func (ps *PubSub) Publish(msg *wrp.Message) error {
	normalized, dest, err := ps.normalize(msg)
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

	for _, route := range routes {
		if _, found := ps.routes[route]; found {
			ps.routes[route].Visit(func(h Handler) {
				// By making this a go routine, we can avoid deadlocks if the handler
				// tries to subscribe to the same service.  It also avoids blocking the
				// caller if the handler takes a long time to process the message.
				if h != nil {
					go h.HandleWrp(*normalized)
				}
			})
		}
	}

	return nil
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
