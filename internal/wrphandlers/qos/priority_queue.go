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
	// MaxMessageBytes is the largest allowable wrp message payload.
	maxMessageBytes int
	// size is the sum of all queued wrp message's payloads
	size int
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
	// Check whether msg violates maxMessageBytes.
	if len(msg.Payload) > pq.maxMessageBytes {
		return
	}

	// Check whether enqueuing msg would violate maxQueueSize.
	// If it does, then drop the least prioritized message.
	// Repeat until the queue no longer violates maxQueueSize.
	for {
		total := pq.size + len(msg.Payload)
		// note, total < 0 checks for an overflow
		if !(total > pq.maxQueueSize || total < 0) || pq.Len() == 0 {
			break
		}

		pq.drop()
	}

	heap.Push(pq, msg)
}

func (pq *priorityQueue) drop() {
	_ = heap.Remove(pq, pq.Len()-1).(wrp.Message)
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
	item := x.(wrp.Message)
	pq.size += len(item.Payload)
	pq.queue = append(pq.queue, item)
}

func (pq *priorityQueue) Pop() any {
	last := len(pq.queue) - 1
	if last < 0 {
		return nil
	}

	item := pq.queue[last]
	pq.size -= len(item.Payload)
	// avoid memory leak
	pq.queue[last] = wrp.Message{}
	pq.queue = pq.queue[0:last]

	return item
}
