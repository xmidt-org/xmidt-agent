// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"container/heap"
	"errors"
	"fmt"
	"time"

	"github.com/xmidt-org/wrp-go/v3"
)

var ErrMaxMessageBytes = errors.New("wrp message payload exceeds maxMessageBytes")

type tieBreaker func(i, j item) bool

type item struct {
	msg       wrp.Message
	timestamp time.Time
	popped    bool
}

type PriorityQueue struct {
	// tieBreaker breaks any QualityOfService ties.
	tieBreaker tieBreaker
	trimQueue  trimPriorityQueue
	// maxQueueBytes is the allowable max size of the queue based on the sum of all queued wrp message's payloads
	maxQueueBytes int64
	// MaxMessageBytes is the largest allowable wrp message payload.
	maxMessageBytes int
	priorityQueue
}

// Dequeue returns the next highest priority message.
func (pq *PriorityQueue) Dequeue() (wrp.Message, bool) {
	var (
		itm item
		ok  bool
	)
	for pq.Len() != 0 {
		itm = heap.Pop(pq).(item)
		// itm.popped will be true if `itm` has already been `trimPriorityQueue.trim()',
		// pop the next item.
		if !itm.popped {
			itm.popped, ok = true, true
			break
		}
	}

	// Keep sizeBytes in sync since both queues point to the same data.
	pq.trimQueue.sizeBytes = pq.sizeBytes

	return itm.msg, ok
}

// Enqueue queues the given message.
func (pq *PriorityQueue) Enqueue(msg wrp.Message) error {
	// Check whether msg violates maxMessageBytes.
	if len(msg.Payload) > pq.maxMessageBytes {
		return fmt.Errorf("%w: %v", ErrMaxMessageBytes, pq.maxMessageBytes)
	}

	item := item{msg: msg, timestamp: time.Now()}
	// Prioritizes messages with the highest QualityOfService.
	heap.Push(pq, &item)
	// Prioritizes messages with the lowest QualityOfService.
	heap.Push(&pq.trimQueue, &item)
	pq.trim()
	return nil
}

// trim removes messages with the lowest QualityOfService until the queue no longer violates `maxQueueSize“.
func (pq *PriorityQueue) trim() {
	// trim until the queue no longer violates maxQueueBytes.
	for pq.trimQueue.sizeBytes > pq.maxQueueBytes {
		// Remove the message with the lowest QualityOfService.
		pq.trimQueue.trim()
	}

	// Keep sizeBytes in sync since both queues point to the same data.
	pq.sizeBytes = pq.trimQueue.sizeBytes
	// Note, `PriorityQueue.Dequeue()' will eventually remove any trimmed items.
}

// heap.Interface related implementation https://pkg.go.dev/container/heap#Interface

func (pq *PriorityQueue) Less(i, j int) bool {
	iItem, jItem := *pq.queue[i], *pq.queue[j]
	iQOS, jQOS := iItem.msg.QualityOfService, jItem.msg.QualityOfService

	// Determine whether a tie breaker is required.
	if iQOS != jQOS {
		return iQOS > jQOS
	}

	return pq.tieBreaker(iItem, jItem)
}

type trimPriorityQueue struct {
	// tieBreaker breaks any QualityOfService ties.
	tieBreaker tieBreaker
	priorityQueue
}

// Dequeue returns the message with the lowest QualityOfService.
func (tpq *trimPriorityQueue) trim() {
	for tpq.Len() != 0 {
		itm := heap.Pop(tpq).(item)
		// itm.popped will be true if `itm` has already been `PriorityQueue.Dequeue()',
		// pop the next item.
		if !itm.popped {
			// Lowest QualityOfService meassage has been trimmed.
			break
		}
	}
}

// heap.Interface related implementation https://pkg.go.dev/container/heap#Interface

// Prioritize messages with the lowest QualityOfService.
func (tpq *trimPriorityQueue) Less(i, j int) bool {
	iItem, jItem := *tpq.queue[i], *tpq.queue[j]
	iQOS, jQOS := iItem.msg.QualityOfService, jItem.msg.QualityOfService

	// Determine whether a tie breaker is required.
	if iQOS != jQOS {
		// Remove messages with the lowest QualityOfService.
		return iQOS < jQOS

	}

	// Tie breaker during queue trimming.
	return tpq.tieBreaker(iItem, jItem)
}

// priorityQueue implements heap.Interface and holds wrp Message, using wrp.QOSValue as its priority.
// https://xmidt.io/docs/wrp/basics/#qos-description-qos
type priorityQueue struct {
	queue []*item
	// sizeBytes is the sum of all queued wrp message's payloads.
	// An int64 overflow is unlikely since that'll be over 9*10^18 bytes
	sizeBytes int64
}

// heap.Interface related implementations https://pkg.go.dev/container/heap#Interface
func (pq *priorityQueue) Len() int { return len(pq.queue) }

func (pq *priorityQueue) Swap(i, j int) {
	pq.queue[i], pq.queue[j] = pq.queue[j], pq.queue[i]
}

func (pq *priorityQueue) Push(x any) {
	item := x.(*item)
	pq.sizeBytes += int64(len(item.msg.Payload))
	pq.queue = append(pq.queue, item)
}

func (pq *priorityQueue) Pop() any {
	last := len(pq.queue) - 1
	if last < 0 {
		return nil
	}

	itm := *pq.queue[last]
	// `pq.queue[last].popped = true`` inform `PriorityQueue.Dequeue()' and `trimPriorityQueue.trim()'
	// that the shared itm has been popped.
	pq.queue[last].popped = true
	pq.sizeBytes -= int64(len(itm.msg.Payload))
	// avoid memory leak (recall is shared pq.queue[last].msg between two queues)
	pq.queue[last].msg = wrp.Message{}
	pq.queue[last] = nil
	pq.queue = pq.queue[0:last]

	return itm
}

func PriorityNewestMsg(i, j item) bool {
	return i.timestamp.After(j.timestamp)
}

func PriorityOldestMsg(i, j item) bool {
	return i.timestamp.Before(j.timestamp)
}
