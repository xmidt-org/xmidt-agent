// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package libparodus

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/pubsub"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/pull"

	// register transports
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNoService    = errors.New("service not found")
)

// This package provides backwards compatibility for the libparodus library.

// Adapter is a struct representing a service that listens for messages from
// libparodus and forwards them to the appropriate service.  It also listens for
// messages from the appropriate service and forwards them to libparodus.
type Adapter struct {
	lock        sync.Mutex
	wg          sync.WaitGroup
	shutdown    context.CancelFunc
	listening   chan error
	subServices map[string]*external

	parodusServiceURL string
	keepaliveInterval time.Duration
	recvTimeout       time.Duration
	sendTimeout       time.Duration
	pubsub            *pubsub.PubSub
}

// Option is the interface implemented by types that can be used to
// configure the service.
type Option interface {
	apply(*Adapter) error
}

type optionFunc func(*Adapter) error

func (f optionFunc) apply(a *Adapter) error {
	return f(a)
}

var _ Option = optionFunc(nil)

// New creates a new Service with the given options.
func New(url string, pubsub *pubsub.PubSub, opts ...Option) (*Adapter, error) {
	required := []Option{
		validatePubSub(),
		validateParodusServiceURL(),
	}

	a := Adapter{
		parodusServiceURL: url,
		pubsub:            pubsub,
		listening:         make(chan error),
		subServices:       make(map[string]*external),
	}

	opts = append(opts, required...)

	for _, o := range opts {
		if o != nil {
			if err := o.apply(&a); err != nil {
				return nil, err
			}
		}
	}

	return &a, nil
}

// Start starts the service.  If the service is already started, this function
// does nothing.  If an error occurs, it is not recoverable and the service can
// not be started.
func (a *Adapter) Start() error {
	var ctx context.Context

	a.lock.Lock()

	if a.shutdown != nil {
		a.lock.Unlock()
		return nil
	}

	ctx, a.shutdown = context.WithCancel(context.Background())

	a.lock.Unlock()

	// Everything beyond this point is run after the lock is released to prevent
	// deadlocks.

	go a.receive(ctx)

	// Wait for the receiver to start listening.
	var err error
	select {
	case err = <-a.listening:
	case <-ctx.Done():
	}

	return err
}

// Stop stops the service.  If the service is already stopped, this function
// does nothing.
func (s *Adapter) Stop() {
	s.lock.Lock()
	shutdown := s.shutdown

	if shutdown != nil {
		shutdown()
	}

	s.shutdown = nil

	s.lock.Unlock()

	for _, ext := range s.subServices {
		ext.cancel()
	}

	s.wg.Wait()
}

// receive listens for messages from libparodus and forwards them to the
// pubsub until context is canceled and the service is stopped.
func (a *Adapter) receive(ctx context.Context) {
	a.wg.Add(1)
	defer a.wg.Done()

	// If we can't create a socket, we can't do anything; exit.
	sock, err := pull.NewSocket()
	if err != nil {
		a.listening <- err
		return
	}

	// Use SetOption to set the receive deadline.  The other ways to set the
	// receive deadline don't seem to work.
	err = sock.SetOption(mangos.OptionRecvDeadline, a.recvTimeout)
	if err != nil {
		a.listening <- err
		return
	}

	// If we can't listen, we can't do anything; exit.
	err = sock.Listen(a.parodusServiceURL)
	if err != nil {
		a.listening <- err
		return
	}

	// Everything is set up and ready to go.  Tell Start() that we're listening.
	a.listening <- nil

	for {
		if ctx.Err() != nil {
			return
		}

		bytes, err := sock.Recv()
		if errors.Is(err, mangos.ErrRecvTimeout) {
			// Ignore read timeouts, but they are important so the routine will
			// eventually exit.
			continue
		}

		var msg wrp.Message
		err = wrp.NewDecoderBytes(bytes, wrp.Msgpack).Decode(&msg)
		if err != nil {
			continue
		}

		switch msg.Type {
		case wrp.ServiceRegistrationMessageType:
			a.register(ctx, msg)
		case wrp.Invalid0MessageType,
			wrp.Invalid1MessageType,
			wrp.ServiceAliveMessageType:
			// Ignore these messages; they should really not be sent to the
			// adapter.  The reason being is the client of this adapter can
			// determine if a service is alive by the presence of the keepalive
			// messages this adapter sends.
			// Simply drop the invalid ones.
			continue
		default:
			_ = a.forward(msg)
		}
	}
}

func (a *Adapter) register(ctx context.Context, msg wrp.Message) error {
	name := msg.ServiceName

	ext, err := newExternal(ctx, name, msg.URL,
		a.keepaliveInterval,
		a.sendTimeout,
		a.pubsub,
		func() {
			a.lock.Lock()
			defer a.lock.Unlock()
			tmp := a.subServices[name]
			if tmp == nil {
				return
			}

			delete(a.subServices, name)
		},
	)

	if err != nil {
		return err
	}

	prev := a.subServices[name]
	if prev != nil {
		prev.cancel()
	}

	a.lock.Lock()
	a.subServices[name] = ext
	a.lock.Unlock()

	return nil
}

func (a *Adapter) forward(msg wrp.Message) error {
	// Send to the pubsub which is responsible for forwarding the message to the
	// appropriate services and/or egress.
	_ = a.pubsub.HandleWrp(msg)

	// The message needs to be sent to the appropriate service attached to this
	// adapter.  The service is determined by the source of the message.
	// Pubsub doesn't handle this because it doesn't know about the services
	// attached to this adapter.
	src, err := wrp.ParseLocator(msg.Source)
	if err != nil {
		return err
	}

	a.lock.Lock()
	sc := a.subServices[src.Service]
	a.lock.Unlock()

	if sc == nil {
		return ErrNoService
	}

	return sc.HandleWrp(msg)
}
