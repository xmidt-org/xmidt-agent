// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"container/heap"

	"github.com/xmidt-org/wrp-go/v3"
)

// PriorityQueue implements heap.Interface and holds wrp Message, using wrp.QOSValue as its priority.
// https://xmidt.io/docs/wrp/basics/#qos-description-qos
type PriorityQueue struct {
	// heap for wrp messages, ingested by serviceQOS
	heap []wrp.Message
	// maxHeapSize is the allowable max size of the queue based on the sum of all queued wrp message's payloads
	maxHeapSize int
}

// Dequeue returns the next highest priority message.
func (pq *PriorityQueue) Dequeue() (wrp.Message, bool) {
	// required, otherwise, heap.Pop will panic during an internal Swap call
	if pq.Len() == 0 {
		return wrp.Message{}, false
	}

	msg, ok := heap.Pop(pq).(wrp.Message)

	return msg, ok
}

// Enqueue queues the given message.
func (pq *PriorityQueue) Enqueue(msg wrp.Message) {
	// check whether msg would already violate maxHeapSize
	if len(msg.Payload) >= pq.maxHeapSize {
		return
	}

	// check whether enqueuing msg would violate maxHeapSize
	// if it does then determine whether to enqueue msg
	// repeat until the heap no longer violates maxHeapSize
	for {
		total := pq.Size() + len(msg.Payload)
		// note, total < 0 checks for a overflow
		if !(total > pq.maxHeapSize || total < 0) || pq.Len() == 0 {
			break
		}

		// determine whether msg is of lower priority than the least
		// prioritized queued message `bottom`
		bottom := heap.Remove(pq, pq.Len()-1).(wrp.Message)
		// if it is then keep bottom, otherwise discard leastMsg and queue msg (the latest)
		if bottom.QualityOfService > msg.QualityOfService {
			msg = bottom
		}
	}

	heap.Push(pq, msg)
}

func (pq *PriorityQueue) Size() int {
	var s int
	for _, msg := range pq.heap {
		s += len(msg.Payload)
	}

	return s
}

func (pq *PriorityQueue) IsFull() bool {
	return len(pq.heap) == cap(pq.heap) || pq.Size() > pq.maxHeapSize
}

// heap.Interface related implementations https://pkg.go.dev/container/heap#Interface

func (pq *PriorityQueue) Len() int { return len(pq.heap) }

func (pq *PriorityQueue) Less(i, j int) bool {
	return pq.heap[i].QualityOfService > pq.heap[j].QualityOfService
}

func (pq *PriorityQueue) Swap(i, j int) {
	pq.heap[i], pq.heap[j] = pq.heap[j], pq.heap[i]
}

func (pq *PriorityQueue) Push(x any) {
	pq.heap = append(pq.heap, x.(wrp.Message))
}

func (pq *PriorityQueue) Pop() any {
	last := len(pq.heap) - 1
	if last < 0 {
		return nil
	}

	item := pq.heap[last]
	// avoid memory leak
	pq.heap[last] = wrp.Message{}
	pq.heap = pq.heap[0:last]

	return item
}
