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
	queue        []wrp.Message
	maxQueueSize int

	m sync.Mutex
}

// Dequeue returns the next highest priority and first-in-line message (FIFO).
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
	pq.m.Lock()
	heap.Push(pq, msg)
	pq.trimQueue()
	pq.m.Unlock()
}

// trimQueue removes the newest messages with lowest priority if the queue is full.
func (pq *PriorityQueue) trimQueue() {
	// when pq.IsFull() is true, pq.Len() > 1 ensures at least 1 message is queued
	for pq.Len() > 1 && pq.IsFull() {
		heap.Remove(pq, pq.Len()-1)
	}
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
	old := pq.queue
	n := len(old)
	if n == 0 {
		return nil
	}

	i := old[n-1]
	// avoid memory leak
	old[n-1] = wrp.Message{}
	pq.queue = old[0 : n-1]

	return i
}
