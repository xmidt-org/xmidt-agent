// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"errors"
	"sync"
	"time"

	"github.com/xmidt-org/wrp-go/v3"
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

// Handler queues incoming messages and sends them to the next wrpkit.Handler
type Handler struct {
	next wrpkit.Handler
	// queue for wrp messages, ingested by serviceQOS
	queue chan wrp.Message
	// priority determines what is used [newest, oldest message] for QualityOfService tie breakers and trimming,
	// with the default being to prioritize the newest messages.
	priority PriorityType
	// tieBreaker breaks any QualityOfService ties.
	tieBreaker tieBreaker
	// maxQueueBytes is the allowable max size of the qos' priority queue, based on the sum of all queued wrp message's payload.
	maxQueueBytes int64
	// MaxMessageBytes is the largest allowable wrp message payload.
	maxMessageBytes int

	// QOS expiries.
	// lowExpires determines when low qos messages are trimmed.
	lowExpires time.Duration
	// mediumExpires determines when medium qos messages are trimmed.
	mediumExpires time.Duration
	// highExpires determines when high qos messages are trimmed.
	highExpires time.Duration
	// criticalExpires determines when critical qos messages are trimmed.
	criticalExpires time.Duration

	lock sync.Mutex
}

// New creates a new instance of the Handler struct.  The parameter next is the
// handler that will be called and monitored for errors.
// Note, once Handler.Stop is called, any calls to Handler.HandleWrp will result in
// an ErrQOSHasShutdown error
func New(next wrpkit.Handler, opts ...Option) (*Handler, error) {
	if next == nil {
		return nil, ErrInvalidInput
	}

	// Add configuration validators.
	opts = append(opts, validateQueueConstraints(), validatePriority(), validateTieBreaker())

	h := Handler{
		next:            next,
		lowExpires:      DefaultLowExpires,
		mediumExpires:   DefaultMediumExpires,
		highExpires:     DefaultHighExpires,
		criticalExpires: DefaultCriticalExpires,
	}

	var errs error
	for _, opt := range opts {
		if opt != nil {
			if err := opt.apply(&h); err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}

	if errs != nil {
		return nil, errs
	}

	return &h, errs
}

func (h *Handler) Start() {
	h.lock.Lock()
	defer h.lock.Unlock()

	if h.queue == nil {
		h.queue = make(chan wrp.Message)
		go h.serviceQOS(h.queue)
	}
}

func (h *Handler) Stop() {
	h.lock.Lock()
	defer h.lock.Unlock()

	if h.queue != nil {
		close(h.queue)
		h.queue = nil
	}
}

// HandleWRP queues incoming messages while the background serviceQOS goroutine attempts
// to send as many queued messages as possible, where the highest QOS messages are prioritized
func (h *Handler) HandleWrp(msg wrp.Message) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	if h.queue == nil {
		return ErrQOSHasShutdown
	}

	h.queue <- msg

	return nil
}

// serviceQOS is a long running goroutine that sends as many queued messages as possible,
// where the highest QOS messages are prioritized.
// Handler.Start starts serviceQOS.
// Handler.Stop stops serviceQOS.
func (h *Handler) serviceQOS(queue <-chan wrp.Message) {
	var (
		// Signaling channel from the handleWRP.
		ready <-chan struct{}
		// Channel for failed deliveries, re-enqueue message.
		failedMsg <-chan wrp.Message
	)

	// create and manage the priority queue
	pq := priorityQueue{
		maxQueueBytes:   h.maxQueueBytes,
		maxMessageBytes: h.maxMessageBytes,
		tieBreaker:      h.tieBreaker,
	}
	for {
		select {
		case msg, ok := <-queue:
			if !ok {
				// Handler.Stop has been called.
				return
			}

			// ErrMaxMessageBytes errrors are ignored.
			_ = pq.Enqueue(msg)
		case <-ready:
			// Previous Handler.wrpHandler has finished, check whether it
			// was successful or not.
			if msg, ok := <-failedMsg; ok {
				// Delivery failed, re-enqueue message and try again later.
				// ErrMaxMessageBytes errrors are ignored.
				_ = pq.Enqueue(msg)
			}

			ready, failedMsg = nil, nil
		}

		if top, ok := pq.Dequeue(); ok {
			failedMsg, ready = h.wrpHandler(top)
		}
	}
}

// wrpHandler calls handler.next.HandleWrp to deliver incoming messages.
// Returns a signaling channel indicating handler.next.HandleWrp is done
// and a message channel for failed deliveries.
func (h *Handler) wrpHandler(msg wrp.Message) (<-chan wrp.Message, <-chan struct{}) {
	ready := make(chan struct{})
	failedMsg := make(chan wrp.Message, 1)
	go func() {
		defer close(ready)
		defer close(failedMsg)

		if err := h.next.HandleWrp(msg); err != nil {
			// Delivery failed, re-enqueue message and try again later.
			failedMsg <- msg
			// The err itself is ignored.
		}
	}()

	return failedMsg, ready
}
