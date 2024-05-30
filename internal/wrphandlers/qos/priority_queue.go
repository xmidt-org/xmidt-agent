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

// priorityQueue implements heap.Interface and holds wrp Message, using wrp.QOSValue as its priority.
// https://xmidt.io/docs/wrp/basics/#qos-description-qos
type priorityQueue struct {
	// queue for wrp messages, ingested by serviceQOS
	queue []item
	// Priority determines what is used [newest, oldest message] for QualityOfService tie breakers,
	// with the default being to prioritize the newest messages.
	priority PriorityType
	// maxQueueBytes is the allowable max size of the queue based on the sum of all queued wrp message's payloads
	maxQueueBytes int64
	// MaxMessageBytes is the largest allowable wrp message payload.
	maxMessageBytes int
	// sizeBytes is the sum of all queued wrp message's payloads.
	// An int64 overflow is unlikely since that'll be over 9*10^18 bytes
	sizeBytes int64
}

type item struct {
	msg       wrp.Message
	timestamp time.Time
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
func (pq *priorityQueue) Enqueue(msg wrp.Message) error {
	// Check whether msg violates maxMessageBytes.
	if len(msg.Payload) > pq.maxMessageBytes {
		return fmt.Errorf("%w: %v", ErrMaxMessageBytes, pq.maxMessageBytes)
	}

	heap.Push(pq, msg)
	pq.trim()
	return nil
}

func (pq *priorityQueue) trim() {
	// trim until the queue no longer violates maxQueueBytes.
	for pq.sizeBytes > pq.maxQueueBytes {
		// Note, priorityQueue.drop does not drop the least prioritized queued message.
		// i.e.: a high priority queued message may be dropped instead of a lesser queued message.
		pq.drop()
	}
}

func (pq *priorityQueue) drop() {
	_ = heap.Remove(pq, pq.Len()-1).(wrp.Message)
}

// heap.Interface related implementations https://pkg.go.dev/container/heap#Interface

func (pq *priorityQueue) Len() int { return len(pq.queue) }

func (pq *priorityQueue) Less(i, j int) bool {
	iQOS := pq.queue[i].msg.QualityOfService
	jQOS := pq.queue[j].msg.QualityOfService
	iTimestamp := pq.queue[i].timestamp
	jTimestamp := pq.queue[j].timestamp

	// Determine what will be used as a priority tie breaker.
	var tieBreaker bool
	switch pt := pq.priority; pt {
	case NewestType:
		// Prioritize the newest messages.
		tieBreaker = iTimestamp.After(jTimestamp)
	case OldestType:
		// Prioritize the oldest messages.
		tieBreaker = iTimestamp.Before(jTimestamp)
	default:
		panic(errors.Join(ErrPriorityTypeInvalid, fmt.Errorf("PriorityType error: '%s' does not match any valid options: %s",
			pt, pt.getKeys())))
	}

	// Determine whether a tie breaker is required.
	if iQOS == jQOS {
		return tieBreaker
	}

	// Prioritize messages with the highest QualityOfService.
	return iQOS > jQOS

}

func (pq *priorityQueue) Swap(i, j int) {
	pq.queue[i], pq.queue[j] = pq.queue[j], pq.queue[i]
}

func (pq *priorityQueue) Push(x any) {
	item := item{msg: x.(wrp.Message), timestamp: time.Now()}
	pq.sizeBytes += int64(len(item.msg.Payload))
	pq.queue = append(pq.queue, item)
}

func (pq *priorityQueue) Pop() any {
	last := len(pq.queue) - 1
	if last < 0 {
		return nil
	}

	msg := pq.queue[last].msg
	pq.sizeBytes -= int64(len(msg.Payload))
	// avoid memory leak
	pq.queue[last] = item{}
	pq.queue = pq.queue[0:last]

	return msg
}
