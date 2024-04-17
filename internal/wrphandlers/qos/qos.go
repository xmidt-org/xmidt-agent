// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"container/heap"
	"context"
	"errors"
	"sync"
	"unsafe"

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
	pq PriorityQueue
	// queue max size
	maxQueueSize int

	m  sync.Mutex
	wg sync.WaitGroup
	// items contain wrp messages sort by descending order (based on wrp message's QOS)
	items chan []bool
	// empty states whether or not the queue is emtpy
	empty chan bool
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
		next:  next,
		items: make(chan []bool, 1),
		empty: make(chan bool, 1),
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
	// add to message
	h.m.Lock()
	defer h.m.Unlock()

	if h.shutdown != nil {
		return
	}

	if len(h.empty) == 0 && len(h.items) == 0 {
		h.empty <- true
	}

	var ctx context.Context
	ctx, h.shutdown = context.WithCancel(context.Background())

	go h.run(ctx)
}

// Stop stops the queue ingestion.
func (h *Handler) Stop() {
	h.m.Lock()
	shutdown := h.shutdown
	// allows qos to restart
	h.shutdown = nil
	if shutdown == nil {
		return
	}
	h.m.Unlock()

	shutdown()
	h.wg.Wait()
}

// run is the long running goroutine used for the queue ingestion.
func (h *Handler) run(ctx context.Context) {
	h.wg.Add(1)
	defer h.wg.Done()
	for i, ok := h.getMsg(ctx); ok; i, ok = h.getMsg(ctx) {
		err := h.next.HandleWrp(*i.value)
		if err != nil {
			h.putPrioritizedMsg(i)
		}
	}
}

// HandleWrp is called to queue a message.
func (h *Handler) HandleWrp(msg wrp.Message) error {
	h.putMsg(&Item{value: &msg})
	return nil
}

// getMsg pops the next highest priority and first-in-line message (FIFO).
func (h *Handler) getMsg(ctx context.Context) (*Item, bool) {
	ok := h.getSignal(ctx)
	if !ok {
		return nil, false
	}

	h.m.Lock()
	// ensures we get the highest priority item/msg
	top, ok := heap.Pop(&h.pq).(*Item)
	h.m.Unlock()
	return top, ok

}

// getSignal receives the signal to pop a message from the queue
func (h *Handler) getSignal(ctx context.Context) bool {
	// consider a getSignal
	var items []bool
	select {
	case <-ctx.Done():
		return false
	case items = <-h.items:
	}

	_ = items[0]
	if len(items) == 1 {
		h.empty <- true
	} else {
		h.items <- items[1:]
	}

	return true
}

// putMsg pushes the message in the queue.
func (h *Handler) putMsg(i *Item) {
	h.m.Lock()
	heap.Push(&h.pq, i)
	h.trimQueue()
	h.m.Unlock()

	h.putSignal()
}

// putPrioritizedMsg pushes the message in the queue as the first-in-line message in order
// to maintain the previous FIFO order of the queue of when the message was originally popped.
// This should be called when a message failed to be forwarded.
func (h *Handler) putPrioritizedMsg(i *Item) {
	h.m.Lock()
	heap.Push(&h.pq, i)
	h.pq.Swap(0, i.index)
	h.trimQueue()
	h.m.Unlock()

	h.putSignal()
}

// putSignal sends the signal to pop a message from the queue
func (h *Handler) putSignal() {
	var items []bool
	select {
	case items = <-h.items:
	case <-h.empty:
	}

	h.items <- append(items, true)
}

// trimQueue pops the message with lowest priority and last in line
func (h *Handler) trimQueue() {
	// when maxQueueSize is 0, len(h.pq) > 1 allows qos to send 1 message at a time
	for len(h.pq) > 1 && (int(unsafe.Sizeof(h.pq)) > h.maxQueueSize) {
		heap.Remove(&h.pq, len(h.pq)-1)
		// remove 1 get signal for every trimmed message
		<-h.items
	}
}
