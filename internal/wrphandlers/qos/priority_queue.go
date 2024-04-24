// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"container/heap"

	"github.com/xmidt-org/wrp-go/v3"
)

// priorityQueue implements heap.Interface and holds wrp Message, using wrp.QOSValue as its priority.
// https://xmidt.io/docs/wrp/basics/#qos-description-qos
type priorityQueue struct {
	// queue for wrp messages, ingested by serviceQOS
	queue []wrp.Message
	// maxQueueSize is the allowable max size of the queue based on the sum of all queued wrp message's payloads
	maxQueueSize int
}

// Dequeue returns the next highest priority message.
func (pq *priorityQueue) Dequeue() (wrp.Message, bool) {
	// Required, otherwise heap.Pop will panic during an internal Swap call.
	if pq.Len() == 0 {
		return wrp.Message{}, false
	}

	msg, ok := heap.Pop(pq).(wrp.Message)

	return msg, ok
}

// Enqueue queues the given message.
func (pq *priorityQueue) Enqueue(msg wrp.Message) {
	// Check whether msg would already violate maxQueueSize.
	if len(msg.Payload) > pq.maxQueueSize {
		return
	}

	// Check whether enqueuing msg would violate maxQueueSize.
	// If it does, then determine whether to enqueue msg.
	// Repeat until the queue no longer violates maxQueueSize.
	for {
		total := pq.Size() + len(msg.Payload)
		// note, total < 0 checks for an overflow
		if !(total > pq.maxQueueSize || total < 0) || pq.Len() == 0 {
			break
		}

		// Determine whether msg is of lower priority than the least
		// prioritized queued message `bottom`.
		bottom := heap.Remove(pq, pq.Len()-1).(wrp.Message)
		// if it is then keep bottom, otherwise discard leastMsg and queue msg (the latest)
		if bottom.QualityOfService > msg.QualityOfService {
			msg = bottom
		}
	}

	heap.Push(pq, msg)
}

func (pq *priorityQueue) Size() int {
	var s int
	for _, msg := range pq.queue {
		s += len(msg.Payload)
	}

	return s
}

// heap.Interface related implementations https://pkg.go.dev/container/heap#Interface

func (pq *priorityQueue) Len() int { return len(pq.queue) }

func (pq *priorityQueue) Less(i, j int) bool {
	return pq.queue[i].QualityOfService > pq.queue[j].QualityOfService
}

func (pq *priorityQueue) Swap(i, j int) {
	pq.queue[i], pq.queue[j] = pq.queue[j], pq.queue[i]
}

func (pq *priorityQueue) Push(x any) {
	pq.queue = append(pq.queue, x.(wrp.Message))
}

func (pq *priorityQueue) Pop() any {
	last := len(pq.queue) - 1
	if last < 0 {
		return nil
	}

	item := pq.queue[last]
	// avoid memory leak
	pq.queue[last] = wrp.Message{}
	pq.queue = pq.queue[0:last]

	return item
}
