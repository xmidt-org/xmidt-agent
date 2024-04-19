// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"context"
	"errors"
	"sync"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/websocket"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

var (
	ErrInvalidInput    = errors.New("invalid input")
	ErrMisconfiguredWS = errors.New("misconfigured WS")
)

// Option is a functional option type for WS.
type Option interface {
	apply(*Handler) error
}

type optionFunc func(*Handler) error

func (f optionFunc) apply(c *Handler) error {
	return f(c)
}

// Handler queues incoming messages and forwards them to the next wrphandler
type Handler struct {
	next wrpkit.Handler
	// queue that'll be used to forward messages to the next wrphandler
	queue PriorityQueue

	m sync.Mutex
	// runEmptyQueue triggers a queue dump, i.e.: sent as many queued messages as possible
	runEmptyQueue chan bool
	// shutdown shuts down the queue ingestion
	shutdown context.CancelFunc
	ctx      context.Context
}

// New creates a new instance of the Handler struct.  The parameter next is the
// handler that will be called and monitored for errors.
func New(next websocket.Egress, opts ...Option) (*Handler, error) {
	if next == nil {
		return nil, ErrInvalidInput
	}

	h := &Handler{
		next:          next,
		runEmptyQueue: make(chan bool, 1),
	}
	for _, opt := range opts {
		if opt != nil {
			if err := opt.apply(h); err != nil {
				return nil, err
			}
		}
	}

	return h, nil
}

// Start starts the queue ingestion and a long running goroutine to maintain
// the queue ingestion.
func (h *Handler) Start() {
	h.m.Lock()
	if h.shutdown != nil {
		return
	}

	// reset
	h.ctx, h.shutdown = context.WithCancel(context.Background())
	h.runEmptyQueue = make(chan bool, 1)
	h.m.Unlock()

	// at qos start, send as many queued messages as possible
	go h.EmptyQueue()

}

// Stop stops the queue ingestion.
func (h *Handler) Stop() {
	h.m.Lock()
	shutdown := h.shutdown
	// allows qos to restart
	close(h.runEmptyQueue)
	h.shutdown = nil
	if shutdown == nil {
		return
	}
	h.m.Unlock()

	shutdown()
}

// HandleWrp is called to queue a message.
func (h *Handler) HandleWrp(msg wrp.Message) error {
	// queue newest message before running runEmptyQueue
	h.queue.Enqueue(msg)
	go h.EmptyQueue()

	return nil
}

// EmptyQueue is the long running goroutine used for the queue ingestion.
func (h *Handler) EmptyQueue() {
	defer func() {
		// trigged runEmptyQueue is done
		<-h.runEmptyQueue
	}()

	h.m.Lock()
	select {
	// Lock() prevents a panic if Stop() called
	case h.runEmptyQueue <- true:
		// sent as many queued messages as possible
	default:
		return
		// a runEmptyQueue is in progress
	}
	ctx := h.ctx
	h.m.Unlock()

	for {
		// always get the next highest priority
		msg, ok := h.queue.Dequeue()
		if !ok {
			// queue is empty
			return
		}

		select {
		case <-ctx.Done():
			// QOS was stopped, re-enqueue message
			h.queue.Enqueue(msg)
			return
		default:
		}

		err := h.next.HandleWrp(msg)
		if err != nil {
			// deliever failed, re-enqueue message
			h.queue.Enqueue(msg)
			return
		}
	}
}
