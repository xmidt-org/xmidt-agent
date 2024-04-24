// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"context"
	"errors"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/websocket"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

var (
	ErrInvalidInput     = errors.New("invalid input")
	ErrMisconfiguredQOS = errors.New("misconfigured QOS")
	ErrQOSHasShutdown   = errors.New("QOS has been shutdown")
)

// Option is a functional option type for QOS.
type Option interface {
	apply(*Handler) error
}

type optionFunc func(*Handler) error

func (f optionFunc) apply(c *Handler) error {
	return f(c)
}

type serviceQOSHandler func(wrp.Message) (<-chan wrp.Message, <-chan struct{})

// Handler queues incoming messages and sends them to the next wrphandler
type Handler struct {
	// queue for wrp messages, ingested by serviceQOS
	queue chan wrp.Message
	// maxQueueSize is the allowable max size of the qos' priority queue, based on the sum of all queued wrp message's payload
	maxQueueSize int
	// done indicates whether or not qos has been shutdown
	done <-chan struct{}
}

// New creates a new instance of the Handler struct.  The parameter next is the
// handler that will be called and monitored for errors.
// Note, once shutdown is called, any calls to Handler.HandleWrp will result in
// an ErrQOSHasShutdown error
func New(next websocket.Egress, opts ...Option) (h *Handler, shutdown func(), err error) {
	if next == nil {
		return nil, nil, ErrInvalidInput
	}

	q := make(chan wrp.Message)
	h = &Handler{
		queue: q,
	}

	for _, opt := range opts {
		if opt != nil {
			if err := opt.apply(h); err != nil {
				return nil, nil, err
			}
		}
	}

	// shutdown() is used to stop serviceQOS by closing its `done` chan.
	ctx, shutdown := context.WithCancel(context.Background())
	h.done = ctx.Done()
	go serviceQOS(ctx.Done(), h.queue, h.maxQueueSize, handleWRPWrapper(next))

	return h, shutdown, nil
}

// HandleWRP queues incoming messages while the background serviceQOS goroutine attempts
// to send as many queued messages as possible, where the highest QOS messages are prioritized
func (h *Handler) HandleWrp(msg wrp.Message) error {
	select {
	// h.done is the same `done <-chan struct{}` that serviceQOS is using
	case <-h.done:
		return ErrQOSHasShutdown
	default:
		// h.queue will never block as long as the serviceQOS goroutine is running.
		h.queue <- msg
	}

	return nil
}

func handleWRPWrapper(next wrpkit.Handler) serviceQOSHandler {
	return func(msg wrp.Message) (<-chan wrp.Message, <-chan struct{}) {
		ready := make(chan struct{})
		failedMsg := make(chan wrp.Message, 1)
		go func() {
			defer close(ready)
			defer close(failedMsg)

			// Note, Websocket.HandleWrp already locks between writes.
			if err := next.HandleWrp(msg); err != nil {
				// Delivery failed, re-enqueue message and try again later.
				failedMsg <- msg
				// The err itself is ignored.
			}
		}()

		return failedMsg, ready
	}
}

// serviceQOS is a long running goroutine that sends as many queued messages as possible,
// where the highest QOS messages are prioritized.
// serviceQOS is stopped when Handler.Cancel() is called, closing the `done` chan.
// Note, New will automatically start the serviceQOS goroutine.
func serviceQOS(done <-chan struct{}, queue <-chan wrp.Message, maxQueueSize int, handleWRP serviceQOSHandler) {
	// create and manage the priority queue
	pq := priorityQueue{maxQueueSize: maxQueueSize}
	var (
		// Signaling channel from the handleWRP.
		ready <-chan struct{}
		// Channel for failed deliveries, re-enqueue message.
		failedMsg <-chan wrp.Message
	)

	for {
		select {
		case <-done:
			return
		case msg := <-queue:
			pq.Enqueue(msg)
			if ready != nil {
				// Previous handleWRP call has not finished, do nothing.
			} else if top, ok := pq.Dequeue(); ok {
				failedMsg, ready = handleWRP(top)
			}
		case <-ready:
			// Previous handleWRP call has finished, check whether handleWRP
			// had successfully delivered its message or not.
			// If it failed, then failedMsg will contain the failed message.
			// Otherwise failedMsg is closed.
			if msg, ok := <-failedMsg; ok {
				// Delivery failed, re-enqueue message and try again later.
				pq.Enqueue(msg)
			}

			ready, failedMsg = nil, nil
			if top, ok := pq.Dequeue(); ok {
				failedMsg, ready = handleWRP(top)
			}
		}
	}
}
