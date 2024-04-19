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
	ErrInvalidInput     = errors.New("invalid input")
	ErrMisconfiguredQOS = errors.New("misconfigured QOS")
)

// Option is a functional option type for QOS.
type Option interface {
	apply(*Handler) error
}

type optionFunc func(*Handler) error

func (f optionFunc) apply(c *Handler) error {
	return f(c)
}

// Handler queues incoming messages and sends them to the next wrphandler
type Handler struct {
	next wrpkit.Handler
	// queue for wrp messages
	queue PriorityQueue

	m sync.Mutex
	// ingestQueue blocks if the queue ingestion is already in progress, i.e.: send
	// as many queued messages as possible
	ingestQueue chan bool
	// shutdown shuts down the queue ingestion
	shutdown context.CancelFunc
}

// New creates a new instance of the Handler struct.  The parameter next is the
// handler that will be called and monitored for errors.
func New(next websocket.Egress, opts ...Option) (*Handler, error) {
	if next == nil {
		return nil, ErrInvalidInput
	}

	h := &Handler{
		next:        next,
		ingestQueue: make(chan bool, 1),
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

// Start starts the qos queue ingestion. Called during a ws connect event.
func (h *Handler) Start() {
	h.m.Lock()
	defer h.m.Unlock()

	if h.shutdown != nil {
		return
	}

	// reset
	var ctx context.Context
	ctx, h.shutdown = context.WithCancel(context.Background())

	go h.run(ctx)

	select {
	case h.ingestQueue <- true:
		// send as many queued messages as possible
	default:
		// queue ingestion is already in progress
	}

}

// Stop stops the qos queue ingestion. Called during a ws disconnect event.
func (h *Handler) Stop() {
	h.m.Lock()
	shutdown := h.shutdown
	// allows qos to restart
	h.shutdown = nil
	h.m.Unlock()

	if shutdown == nil {
		return
	}

	shutdown()
}

// HandleWrp is called to queue a message and then attempt to send as many queued messages as possible.
func (h *Handler) HandleWrp(msg wrp.Message) error {
	// queue newest message
	h.queue.Enqueue(msg)
	select {
	case h.ingestQueue <- true:
		// send as many queued messages as possible
	default:
		// queue ingestion is already in progress
	}

	return nil
}

// run sends as many queued messages as possible until the following occurs:
// 1. the qos context was cancelled
// 2. qos queue is empty
// 3. there was a delivery failure
// Note, Start() will automatically trigger a SendQueuedMessages().
func (h *Handler) run(ctx context.Context) {
	for {
		select {
		case <-h.ingestQueue:
		// queue ingestion will start
		case <-ctx.Done():
			// QOS was stopped, exit
			return
		}

		for {
			// always get the next highest priority message
			msg, ok := h.queue.Dequeue()
			if !ok {
				// queue is empty, wait for next ingestQueue trigger
				break
			}

			select {
			case <-ctx.Done():
				// QOS was stopped, re-enqueue message and exit
				// run() loop will restart on a connect event
				h.queue.Enqueue(msg)
				return
			default:
			}

			err := h.next.HandleWrp(msg)
			if err != nil {
				// delivery failed, re-enqueue message and wait for next ingestQueue trigger
				h.queue.Enqueue(msg)
				break
			}
		}
	}
}
