// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"container/heap"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/xmidt-org/wrp-go/v5"
)

var ErrMaxMessageBytes = errors.New("wrp message payload exceeds maxMessageBytes")

const (
	// https://xmidt.io/docs/wrp/basics/#request-delivery-response-rdr-codes
	messageIsTooLarge                int64 = 4
	higherPriorityMessageTookTheSpot int64 = 102
)

// priorityQueue implements heap.Interface and holds wrp Message, using wrp.QOSValue as its priority.
// https://xmidt.io/docs/wrp/basics/#qos-description-qos
type priorityQueue struct {
	// queue for wrp messages, ingested by serviceQOS
	queue []item
	// Priority determines what is used [newest, oldest message] for QualityOfService tie breakers and trimming,
	// with the default being to prioritize the newest messages.
	priority PriorityType
	// tieBreaker breaks any QualityOfService ties.
	tieBreaker tieBreaker
	// maxQueueBytes is the allowable max size of the queue based on the sum of all queued wrp message's payloads.
	// Zero value will disable individual message size validation.
	maxQueueBytes int64
	// MaxMessageBytes is the largest allowable wrp message payload.
	maxMessageBytes int
	// sizeBytes is the sum of all queued wrp message's payloads.
	// An int64 overflow is unlikely since that'll be over 9*10^18 bytes
	sizeBytes int64

	// QOS expiries.
	// lowExpires determines when low qos messages are trimmed.
	lowExpires time.Duration
	// mediumExpires determines when medium qos messages are trimmed.
	mediumExpires time.Duration
	// highExpires determines when high qos messages are trimmed.
	highExpires time.Duration
	// criticalExpires determines when critical qos messages are trimmed.
	criticalExpires time.Duration
}

type tieBreaker func(i, j item) bool

type item struct {
	// msg is the message queued for delivery.
	msg *wrp.Message
	// expires is the time the messge is good upto before it is eligible to be trimmed.
	expires time.Time
	// discard determines whether a message should be discarded or not
	discard bool
}

func (itm *item) dispose() (payloadSize int64) {
	var rdr = higherPriorityMessageTookTheSpot

	payloadSize = int64(len(itm.msg.Payload))
	// Mark itm to be discarded.
	itm.discard = true
	// Preemptively discard itm's payload to reduce
	// resource usage, since itm will be discarded,
	itm.msg.Payload = nil
	itm.msg.RequestDeliveryResponse = &rdr

	return payloadSize
}

// Dequeue returns the next highest priority message.
func (pq *priorityQueue) Dequeue() (msg wrp.Message, ok bool) {
	if pq.Len() == 0 {
		return msg, false
	}

	itm, ok := heap.Pop(pq).(item)
	if ok {
		msg = *itm.msg
	}

	// ok will be false if no message was found, otherwise ok will be true.
	return msg, ok
}

// Enqueue queues the given message.
func (pq *priorityQueue) Enqueue(msg wrp.Message) error {
	var err error

	// Check whether msg violates maxMessageBytes.
	// The zero value of `pq.maxMessageBytes` will disable individual message size validation.
	if pq.maxMessageBytes != 0 && len(msg.Payload) > pq.maxMessageBytes {
		var rdr = messageIsTooLarge

		msg.Payload = nil
		msg.RequestDeliveryResponse = &rdr
		err = fmt.Errorf("%w: %v", ErrMaxMessageBytes, pq.maxMessageBytes)
	}

	heap.Push(pq, msg)
	pq.trim()

	return err
}

// trim removes messages with the lowest QualityOfService until the queue no longer violates `maxQueueSize“.
func (pq *priorityQueue) trim() {
	// If priorityQueue.queue doesn't violates `maxQueueSize`, then return.
	if pq.sizeBytes <= pq.maxQueueBytes {
		return
	}

	itemsCache := make([]*item, len(pq.queue))
	// Remove all expired messages before trimming unexpired lower priority messages.
	now := time.Now()
	iCache := 0
	for i := range pq.queue {
		itm := &pq.queue[i]
		// itm has already been marked to be discarded.
		if itm.discard {
			continue
		}
		if now.After(itm.expires) {
			// Mark itm to be discarded.
			pq.sizeBytes -= itm.dispose()
			continue
		}

		itemsCache[iCache] = itm
		iCache += 1
	}

	// Resize itemsCache.
	itemsCache = itemsCache[:iCache]
	slices.SortFunc(itemsCache, func(i, j *item) int {
		if i.msg.QualityOfService < j.msg.QualityOfService {
			return -1
		} else if i.msg.QualityOfService > j.msg.QualityOfService {
			return 1
		}

		// Tiebreaker.
		switch pq.priority {
		case NewestType:
			// Prioritize the newest messages.
			return i.expires.Compare(j.expires)
		default:
			// Prioritize the oldest messages.
			return j.expires.Compare(i.expires)
		}
	})

	// Continue trimming until the pq.queue no longer violates maxQueueBytes.
	// Remove the messages with the lowest priority.
	for _, itm := range itemsCache {
		// If pq.queue doesn't violates `maxQueueSize`, then return.
		if pq.sizeBytes <= pq.maxQueueBytes {
			break
		}

		// Mark itm to be discarded.
		pq.sizeBytes -= itm.dispose()
	}

}

// heap.Interface related implementations https://pkg.go.dev/container/heap#Interface

func (pq *priorityQueue) Len() int { return len(pq.queue) }

func (pq *priorityQueue) Less(i, j int) bool {
	iItem, jItem := pq.queue[i], pq.queue[j]
	iQOS, jQOS := iItem.msg.QualityOfService, jItem.msg.QualityOfService

	// Determine whether a tie breaker is required.
	if iQOS != jQOS {
		return iQOS > jQOS
	}

	return pq.tieBreaker(iItem, jItem)
}

func (pq *priorityQueue) Swap(i, j int) {
	pq.queue[i], pq.queue[j] = pq.queue[j], pq.queue[i]
}

func (pq *priorityQueue) Push(x any) {
	msg := x.(wrp.Message)
	pq.sizeBytes += int64(len(msg.Payload))

	var qosExpires time.Duration
	switch msg.QualityOfService.Level() {
	case wrp.QOSLow:
		qosExpires = pq.lowExpires
	case wrp.QOSMedium:
		qosExpires = pq.mediumExpires
	case wrp.QOSHigh:
		qosExpires = pq.highExpires
	case wrp.QOSCritical:
		qosExpires = pq.criticalExpires
	}

	pq.queue = append(pq.queue, item{
		msg:     &msg,
		expires: time.Now().Add(qosExpires),
		discard: false})
}

func (pq *priorityQueue) Pop() any {
	last := len(pq.queue) - 1
	if last < 0 {
		return nil
	}

	itm := pq.queue[last]
	pq.sizeBytes -= int64(len(itm.msg.Payload))
	// avoid memory leak
	pq.queue[last] = item{}
	pq.queue = pq.queue[0:last]

	return itm
}

func PriorityNewestMsg(i, j item) bool {
	return i.expires.After(j.expires)
}

func PriorityOldestMsg(i, j item) bool {
	return i.expires.Before(j.expires)
}
