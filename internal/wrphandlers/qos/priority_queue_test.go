// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/wrp-go/v3"
)

func TestPriorityQueue(t *testing.T) {
	tests := []struct {
		description string
		test        func(*testing.T)
	}{
		{"Enqueue and Dequeue", testEnqueueDequeue},
		{"Enqueue and Dequeue with age priority", testEnqueueDequeueAgePriority},
		{"Size", testSize},
		{"Len", testLen},
		{"Less", testLess},
		{"Swap", testSwap},
		{"Push", testPush},
		{"Pop", testPop},
	}

	for _, tc := range tests {
		t.Run(tc.description, tc.test)
	}
}

func testEnqueueDequeueAgePriority(t *testing.T) {
	smallLowQOSMsgNewest := wrp.Message{
		Destination:      "mac:00deadbeef00/config",
		Payload:          []byte("{}"),
		QualityOfService: wrp.QOSLowValue,
	}
	smallLowQOSMsgOldest := wrp.Message{
		Destination:      "mac:00deadbeef01/config",
		Payload:          []byte("{}"),
		QualityOfService: wrp.QOSLowValue,
	}
	messages := []wrp.Message{
		smallLowQOSMsgOldest,
		smallLowQOSMsgNewest,
		smallLowQOSMsgNewest,
		smallLowQOSMsgNewest,
		smallLowQOSMsgNewest,
	}
	tests := []struct {
		description string
		tieBreaker  tieBreaker
		expectedMsg wrp.Message
	}{
		{
			description: "drop incoming low priority messages while prioritizing older messages",
			tieBreaker:  PriorityOldestMsg,
			expectedMsg: smallLowQOSMsgOldest,
		},
		{
			description: "drop incoming low priority messages while prioritizing newer messages",
			tieBreaker:  PriorityNewestMsg,
			expectedMsg: smallLowQOSMsgNewest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)
			pq := priorityQueue{
				maxQueueBytes:   int64(len(smallLowQOSMsgOldest.Payload)),
				maxMessageBytes: len(smallLowQOSMsgOldest.Payload),
				tieBreaker:      tc.tieBreaker,
			}
			for _, msg := range messages {
				pq.Enqueue(msg)
			}

			assert.Equal(1, pq.Len())

			actualMsg, ok := pq.Dequeue()
			require.True(ok)
			require.NotEmpty(actualMsg)
			assert.Equal(tc.expectedMsg, actualMsg)
		})
	}
}

func testEnqueueDequeue(t *testing.T) {
	emptyLowQOSMsg := wrp.Message{
		Destination:      "mac:00deadbeef00/config",
		QualityOfService: wrp.QOSLowValue,
	}
	smallLowQOSMsg := wrp.Message{
		Destination:      "mac:00deadbeef01/config",
		Payload:          []byte("{}"),
		QualityOfService: wrp.QOSLowValue,
	}
	mediumMediumQosMsg := wrp.Message{
		Destination:      "mac:00deadbeef02/config",
		Payload:          []byte("{\"command\":\"GET\",\"names\":[]}"),
		QualityOfService: wrp.QOSMediumValue,
	}
	largeCriticalQOSMsg := wrp.Message{
		Destination:      "mac:00deadbeef03/config",
		Payload:          []byte("{\"command\":\"GET\",\"names\":[\"NoSuchParameter\"]}"),
		QualityOfService: wrp.QOSCriticalValue,
	}
	enqueueSequenceTest := []wrp.Message{
		largeCriticalQOSMsg,
		mediumMediumQosMsg,
		smallLowQOSMsg,
		largeCriticalQOSMsg,
		smallLowQOSMsg,
		largeCriticalQOSMsg,
		smallLowQOSMsg,
		mediumMediumQosMsg,
	}
	dequeueSequenceTest := []wrp.Message{
		largeCriticalQOSMsg,
		largeCriticalQOSMsg,
		largeCriticalQOSMsg,
		mediumMediumQosMsg,
		mediumMediumQosMsg,
		smallLowQOSMsg,
		smallLowQOSMsg,
		smallLowQOSMsg,
	}

	var queueSizeSequenceTest int
	for _, msg := range enqueueSequenceTest {
		queueSizeSequenceTest += len(msg.Payload)
	}

	tests := []struct {
		description             string
		messages                []wrp.Message
		maxQueueBytes           int
		maxMessageBytes         int
		expectedQueueSize       int
		expectedDequeueSequence []wrp.Message
	}{
		// success cases
		{
			description: "allows enqueue messages without payloads",
			messages: []wrp.Message{
				// nonempty payload
				smallLowQOSMsg,
				// empty payloads
				emptyLowQOSMsg,
				emptyLowQOSMsg,
				emptyLowQOSMsg,
				emptyLowQOSMsg},
			maxQueueBytes:     len(smallLowQOSMsg.Payload),
			maxMessageBytes:   len(smallLowQOSMsg.Payload),
			expectedQueueSize: 5,
		},
		{
			description:       "message too large with an empty queue",
			messages:          []wrp.Message{largeCriticalQOSMsg},
			maxQueueBytes:     len(smallLowQOSMsg.Payload),
			maxMessageBytes:   len(largeCriticalQOSMsg.Payload),
			expectedQueueSize: 0,
		},
		{
			description:       "allow unbound queue size",
			messages:          []wrp.Message{largeCriticalQOSMsg, largeCriticalQOSMsg},
			maxMessageBytes:   len(largeCriticalQOSMsg.Payload),
			expectedQueueSize: 2,
		},
		{
			description:       "message too large with a nonempty queue",
			messages:          []wrp.Message{largeCriticalQOSMsg, largeCriticalQOSMsg},
			maxQueueBytes:     len(largeCriticalQOSMsg.Payload),
			maxMessageBytes:   len(largeCriticalQOSMsg.Payload),
			expectedQueueSize: 1,
		},
		{
			description:       "remove some low priority messages to fit a higher priority message",
			messages:          []wrp.Message{mediumMediumQosMsg, mediumMediumQosMsg, mediumMediumQosMsg, largeCriticalQOSMsg},
			maxQueueBytes:     len(mediumMediumQosMsg.Payload) * 3,
			maxMessageBytes:   len(largeCriticalQOSMsg.Payload),
			expectedQueueSize: 2,
		},
		{
			description:       "remove all low priority messages to fit a higher priority message",
			messages:          []wrp.Message{smallLowQOSMsg, smallLowQOSMsg, smallLowQOSMsg, smallLowQOSMsg, largeCriticalQOSMsg},
			maxQueueBytes:     len(largeCriticalQOSMsg.Payload),
			maxMessageBytes:   len(largeCriticalQOSMsg.Payload),
			expectedQueueSize: 1,
		},
		{
			description:             "dequeue all messages from highest to lowest priority",
			messages:                enqueueSequenceTest,
			maxQueueBytes:           queueSizeSequenceTest,
			maxMessageBytes:         len(largeCriticalQOSMsg.Payload),
			expectedQueueSize:       len(enqueueSequenceTest),
			expectedDequeueSequence: dequeueSequenceTest,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)
			pq := priorityQueue{
				maxQueueBytes:   int64(tc.maxQueueBytes),
				maxMessageBytes: tc.maxMessageBytes,
				tieBreaker:      PriorityNewestMsg,
			}
			for _, msg := range tc.messages {
				pq.Enqueue(msg)
			}

			assert.Equal(tc.expectedQueueSize, pq.Len())

			if len(tc.expectedDequeueSequence) == 0 {
				return
			}

			for _, expectedMsg := range tc.expectedDequeueSequence {
				actualMsg, ok := pq.Dequeue()
				require.True(ok)
				require.NotEmpty(actualMsg)
				assert.Equal(expectedMsg, actualMsg)
			}
		})
	}
}

func testSize(t *testing.T) {
	assert := assert.New(t)
	msg := wrp.Message{
		Destination: "mac:00deadbeef00/config",
		Payload:     []byte("{\"command\":\"GET\",\"names\":[\"NoSuchParameter\"]}"),
	}
	pq := priorityQueue{tieBreaker: PriorityNewestMsg}

	assert.Equal(int64(0), pq.sizeBytes)
	pq.Push(msg)
	pq.Push(msg)
	assert.Equal(int64(len(msg.Payload)*2), pq.sizeBytes)
}
func testLen(t *testing.T) {
	assert := assert.New(t)
	pq := priorityQueue{queue: []item{
		{
			msg: wrp.Message{

				Destination: "mac:00deadbeef00/config",
			},
			timestamp: time.Now(),
		}, {
			msg: wrp.Message{
				Destination: "mac:00deadbeef01/config",
			},
			timestamp: time.Now(),
		},
	},
		tieBreaker: PriorityNewestMsg,
	}

	assert.Equal(len(pq.queue), pq.Len())
}

func testLess(t *testing.T) {
	oldestMsg := item{
		msg: wrp.Message{
			Destination:      "mac:00deadbeef00/config",
			QualityOfService: wrp.QOSCriticalValue,
		},
		timestamp: time.Now(),
	}
	newestMsg := item{
		msg: wrp.Message{
			Destination:      "mac:00deadbeef01/config",
			QualityOfService: wrp.QOSLowValue,
		},
		timestamp: time.Now(),
	}
	tieBreakerMsg := item{
		msg: wrp.Message{
			Destination:      "mac:00deadbeef02/config",
			QualityOfService: wrp.QOSCriticalValue,
		},
		timestamp: time.Now(),
	}
	tests := []struct {
		description string
		priority    PriorityType
		tieBreaker  tieBreaker
	}{
		{
			description: "less",
			priority:    NewestType,
			tieBreaker:  PriorityNewestMsg,
		},
		{
			description: "tie breaker prioritizing newer messages",
			priority:    NewestType,
			tieBreaker:  PriorityNewestMsg,
		},
		{
			description: "tie breaker prioritizing older messages",
			priority:    OldestType,
			tieBreaker:  PriorityOldestMsg,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			pq := priorityQueue{
				queue:      []item{oldestMsg, newestMsg, tieBreakerMsg},
				tieBreaker: tc.tieBreaker,
			}

			// wrp.QOSCriticalValue > wrp.QOSLowValue
			assert.True(pq.Less(0, 1))
			// wrp.QOSLowValue > wrp.QOSCriticalValue
			assert.False(pq.Less(1, 0))

			// Tie Breakers.
			switch tc.priority {
			case NewestType:
				// tieBreakerMsg [2] is the newest message.
				assert.False(pq.Less(0, 2))
				assert.True(pq.Less(2, 0))
			case OldestType:
				// oldestMsg [0] is the oldest message.
				assert.True(pq.Less(0, 2))
				assert.False(pq.Less(2, 0))
			}
		})
	}
}

func testSwap(t *testing.T) {
	assert := assert.New(t)
	msg0 := wrp.Message{
		Destination: "mac:00deadbeef00/config",
	}
	msg2 := wrp.Message{
		Destination: "mac:00deadbeef02/config",
	}
	pq := priorityQueue{queue: []item{
		{
			msg:       msg0,
			timestamp: time.Now(),
		},
		{
			msg: wrp.Message{
				Destination: "mac:00deadbeef01/config",
			},
			timestamp: time.Now(),
		},
		{
			msg:       msg2,
			timestamp: time.Now(),
		},
	},
		tieBreaker: PriorityNewestMsg,
	}

	pq.Swap(0, 2)
	// pq.queue[0] should contain msg2
	assert.Equal(msg2, pq.queue[0].msg)
	// pq.queue[2] should contain msg0
	assert.Equal(msg0, pq.queue[2].msg)
}

func testPush(t *testing.T) {
	assert := assert.New(t)
	messages := []wrp.Message{
		{
			Destination: "mac:00deadbeef00/config",
		},
		{
			Destination: "mac:00deadbeef01/config",
		},
		{
			Destination: "mac:00deadbeef02/config",
		},
	}
	pq := priorityQueue{tieBreaker: PriorityNewestMsg}
	for _, msg := range messages {
		pq.Push(msg)
		assert.Equal(msg, pq.queue[pq.Len()-1].msg)
	}
}

func testPop(t *testing.T) {
	msg0 := wrp.Message{
		Destination: "mac:00deadbeef00/config",
	}
	msg1 := wrp.Message{
		Destination: "mac:00deadbeef01/config",
	}
	msg2 := wrp.Message{
		Destination: "mac:00deadbeef02/config",
	}
	tests := []struct {
		description     string
		items           []item
		expectedMessage wrp.Message
	}{
		// success cases
		{
			description: "empty queue",
		},
		{
			description: "single message with memory leak check",
			items: []item{
				{
					msg: msg0,
				},
			},
			expectedMessage: msg0,
		},
		{
			description: "multiple messages with memory leak check",
			items: []item{
				{
					msg: msg0,
				},
				{
					msg: msg1,
				},
				{
					msg: msg2,
				},
			},
			expectedMessage: msg2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			pq := priorityQueue{queue: tc.items, tieBreaker: PriorityNewestMsg}
			// no sorting is applied, Pop will pop the last message from priorityQueue's queue
			switch msg := pq.Pop().(type) {
			case nil:
				assert.Len(tc.items, 0)
			case wrp.Message:
				assert.Equal(tc.expectedMessage, msg)
				require.NotEmpty(tc.items, "Pop() should have returned a nil instead of a wrp.Message")
				// check for memory leak
				assert.Empty(tc.items[len(tc.items)-1])
				assert.Equal(wrp.Message{}, tc.items[len(tc.items)-1].msg)
				assert.True(tc.items[len(tc.items)-1].timestamp.IsZero())
			default:
				require.Fail("Pop() returned an unknown type")
			}
		})
	}
}
