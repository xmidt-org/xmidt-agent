// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"container/heap"
	"sync"

	"github.com/xmidt-org/wrp-go/v3"
)

// PriorityQueue implements heap.Interface and holds wrp Message, using wrp.QOSValue as its priority.
// https://xmidt.io/docs/wrp/basics/#qos-description-qos
type PriorityQueue struct {
	queue []wrp.Message
	// maxQueueSize is the allowable max size of the queue based on the sum of all queued wrp message's payloads
	maxQueueSize int

	m sync.Mutex
}

// Dequeue returns the next highest priority message.
func (pq *PriorityQueue) Dequeue() (wrp.Message, bool) {
	defer pq.m.Unlock()
	pq.m.Lock()
	if pq.Len() == 0 {
		return wrp.Message{}, false
	}

	msg, ok := heap.Pop(pq).(wrp.Message)

	return msg, ok

}

// Enqueue queues the given message.
func (pq *PriorityQueue) Enqueue(msg wrp.Message) {
	defer pq.m.Unlock()
	pq.m.Lock()
	// when the queue is empty, check whether enqueuing msg would violate maxQueueSize
	if pq.Len() == 0 {
		if len(msg.Payload) <= pq.maxQueueSize {
			heap.Push(pq, msg)
		}

		return
	}

	// check whether enqueuing msg would violate the queue constraints
	// if it does then determine whether to enqueue msg, otherwise enqueue msg
	if pq.Size()+len(msg.Payload) > pq.maxQueueSize || len(pq.queue)+1 > cap(pq.queue) {
		// determine whether msg is of lower priority than the least
		// prioritized queued message `leastMsg`
		leastMsg := heap.Remove(pq, pq.Len()-1).(wrp.Message)
		// if it is then keep leastMsg, otherwise discard leastMsg and queue msg (the latest)
		if leastMsg.QualityOfService > msg.QualityOfService {
			msg = leastMsg
		}
	}

	heap.Push(pq, msg)
}

func (pq *PriorityQueue) Size() int {
	var s int
	for _, msg := range pq.queue {
		s += len(msg.Payload)
	}

	return s
}

func (pq *PriorityQueue) IsFull() bool {
	return len(pq.queue) == cap(pq.queue) || pq.Size() > pq.maxQueueSize
}

// heap.Interface related implementations https://pkg.go.dev/container/heap#Interface

func (pq *PriorityQueue) Len() int { return len(pq.queue) }

func (pq *PriorityQueue) Less(i, j int) bool {
	return pq.queue[i].QualityOfService > pq.queue[j].QualityOfService
}

func (pq *PriorityQueue) Swap(i, j int) {
	pq.queue[i], pq.queue[j] = pq.queue[j], pq.queue[i]
}

func (pq *PriorityQueue) Push(x any) {
	pq.queue = append(pq.queue, x.(wrp.Message))
}

func (pq *PriorityQueue) Pop() any {
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
