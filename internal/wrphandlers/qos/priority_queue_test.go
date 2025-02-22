// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/wrp-go/v5"
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
		{"Trim", testTrim},
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
		priority    PriorityType
		expectedMsg wrp.Message
	}{
		{
			description: "drop incoming low priority messages while prioritizing older messages",
			priority:    OldestType,
			expectedMsg: smallLowQOSMsgOldest,
		},
		{
			description: "drop incoming low priority messages while prioritizing newer messages",
			priority:    NewestType,
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
				lowExpires:      DefaultLowExpires,
				mediumExpires:   DefaultMediumExpires,
				highExpires:     DefaultHighExpires,
				criticalExpires: DefaultCriticalExpires,
				priority:        tc.priority,
			}

			var err error
			pq.tieBreaker, err = priority(tc.priority)
			require.NoError(err)

			for _, msg := range messages {
				err = pq.Enqueue(msg)
				if len(msg.Payload) > pq.maxMessageBytes && pq.maxMessageBytes != 0 {
					assert.Error(err)
				} else {
					assert.NoError(err)
				}
			}

			actualMsg, ok := pq.Dequeue()
			require.True(ok)
			require.NotEmpty(actualMsg)
			assert.Equal(tc.expectedMsg, actualMsg)
			assert.Equal(int64(0), pq.sizeBytes)
		})
	}
}

func testEnqueueDequeue(t *testing.T) {
	var rdr = messageIsTooLarge
	emptyLowQOSMsg := wrp.Message{
		Destination:      "mac:00deadbeef00/config",
		QualityOfService: 10,
	}
	smallLowQOSMsg := wrp.Message{
		Destination:      "mac:00deadbeef01/config",
		Payload:          []byte("{}"),
		QualityOfService: 10,
	}
	mediumMediumQosMsg := wrp.Message{
		Destination:      "mac:00deadbeef02/config",
		Payload:          []byte("{\"command\":\"GET\",\"names\":[]}"),
		QualityOfService: wrp.QOSMediumValue,
	}
	mediumHighQosMsg := wrp.Message{
		Destination:      "mac:00deadbeef02/config",
		Payload:          []byte("{\"command\":\"GET\",\"names\":[]}"),
		QualityOfService: wrp.QOSHighValue,
	}
	largeCriticalQOSMsg := wrp.Message{
		Destination:      "mac:00deadbeef03/config",
		Payload:          []byte("{\"command\":\"GET\",\"names\":[\"NoSuchParameter\"]}"),
		QualityOfService: wrp.QOSCriticalValue,
	}
	xLargeCriticalQOSMsg := wrp.Message{
		Destination:      "mac:00deadbeef04/config",
		Payload:          []byte("{\"command\":\"GET\",\"names\":[\"NoSuchParameterXL\"]}"),
		QualityOfService: wrp.QOSCriticalValue,
	}
	emptyXLargeCriticalQOSMsg := wrp.Message{
		Destination:             "mac:00deadbeef04/config",
		QualityOfService:        wrp.QOSCriticalValue,
		RequestDeliveryResponse: &rdr,
	}
	enqueueSequenceTest := []wrp.Message{
		mediumMediumQosMsg,
		smallLowQOSMsg,
		largeCriticalQOSMsg,
		smallLowQOSMsg,
		largeCriticalQOSMsg,
		smallLowQOSMsg,
		mediumMediumQosMsg,
		mediumHighQosMsg,
	}
	dequeueSequenceTest := []wrp.Message{
		largeCriticalQOSMsg,
		largeCriticalQOSMsg,
		mediumHighQosMsg,
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

	// test message payload drop
	enqueueSequenceTest = append(enqueueSequenceTest, xLargeCriticalQOSMsg)
	dequeueSequenceTest = append([]wrp.Message{emptyXLargeCriticalQOSMsg}, dequeueSequenceTest...)

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
			maxQueueBytes:     len(largeCriticalQOSMsg.Payload),
			maxMessageBytes:   len(largeCriticalQOSMsg.Payload) - 1,
			expectedQueueSize: 0,
		},
		{
			description:       "allow any message size",
			messages:          []wrp.Message{largeCriticalQOSMsg},
			maxQueueBytes:     len(largeCriticalQOSMsg.Payload),
			expectedQueueSize: 1,
		},
		{
			description:             "message too large with a nonempty queue",
			messages:                []wrp.Message{largeCriticalQOSMsg, xLargeCriticalQOSMsg},
			maxQueueBytes:           len(largeCriticalQOSMsg.Payload),
			maxMessageBytes:         len(largeCriticalQOSMsg.Payload),
			expectedQueueSize:       1,
			expectedDequeueSequence: []wrp.Message{emptyXLargeCriticalQOSMsg, largeCriticalQOSMsg},
		},
		{
			description:             "drop incoming low priority messages",
			messages:                []wrp.Message{largeCriticalQOSMsg, largeCriticalQOSMsg, smallLowQOSMsg, mediumMediumQosMsg},
			maxQueueBytes:           len(largeCriticalQOSMsg.Payload) * 2,
			maxMessageBytes:         len(largeCriticalQOSMsg.Payload),
			expectedQueueSize:       2,
			expectedDequeueSequence: []wrp.Message{largeCriticalQOSMsg, largeCriticalQOSMsg},
		},
		{
			description:       "remove some low priority messages to fit a higher priority message",
			messages:          []wrp.Message{mediumMediumQosMsg, smallLowQOSMsg, mediumMediumQosMsg, smallLowQOSMsg, mediumMediumQosMsg, smallLowQOSMsg, largeCriticalQOSMsg, smallLowQOSMsg, mediumMediumQosMsg},
			maxQueueBytes:     len(mediumMediumQosMsg.Payload)*2 + len(largeCriticalQOSMsg.Payload),
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
				lowExpires:      DefaultLowExpires,
				mediumExpires:   DefaultMediumExpires,
				highExpires:     DefaultHighExpires,
				criticalExpires: DefaultCriticalExpires,
			}

			var err error
			pq.tieBreaker, err = priority(NewestType)
			require.NoError(err)

			for _, msg := range tc.messages {
				err = pq.Enqueue(msg)
				if len(msg.Payload) > pq.maxMessageBytes && pq.maxMessageBytes != 0 {
					assert.Error(err)
				} else {
					assert.NoError(err)
				}
			}

			if len(tc.expectedDequeueSequence) == 0 {
				return
			}

			for _, expectedMsg := range tc.expectedDequeueSequence {
				actualMsg, ok := pq.Dequeue()
				require.True(ok)
				require.NotEmpty(actualMsg)
				assert.Equal(expectedMsg, actualMsg)
			}

			assert.Equal(int64(0), pq.sizeBytes)

		})
	}
}

func testSize(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	msg := wrp.Message{
		Destination: "mac:00deadbeef00/config",
		Payload:     []byte("{\"command\":\"GET\",\"names\":[\"NoSuchParameter\"]}"),
	}
	pq := priorityQueue{}

	var err error
	pq.tieBreaker, err = priority(NewestType)
	require.NoError(err)

	assert.Equal(int64(0), pq.sizeBytes)
	pq.Push(msg)
	pq.Push(msg)
	assert.Equal(int64(len(msg.Payload)*2), pq.sizeBytes)
}

func testLen(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	pq := priorityQueue{}
	pq.queue = []item{
		{
			msg: &wrp.Message{
				Destination: "mac:00deadbeef00/config",
			},
			expires: time.Now(),
		}, {
			msg: &wrp.Message{
				Destination: "mac:00deadbeef01/config",
			},
			expires: time.Now(),
		},
	}

	var err error
	pq.tieBreaker, err = priority(NewestType)
	require.NoError(err)
	assert.Equal(len(pq.queue), pq.Len())
}

func testLess(t *testing.T) {
	oldestMsg := item{
		msg: &wrp.Message{
			Destination:      "mac:00deadbeef00/config",
			QualityOfService: wrp.QOSCriticalValue,
		},
		expires: time.Now(),
	}
	newestMsg := item{
		msg: &wrp.Message{
			Destination:      "mac:00deadbeef01/config",
			QualityOfService: wrp.QOSLowValue,
		},
		expires: time.Now(),
	}
	tieBreakerMsg := item{
		msg: &wrp.Message{
			Destination:      "mac:00deadbeef02/config",
			QualityOfService: wrp.QOSCriticalValue,
		},
		expires: time.Now(),
	}
	tests := []struct {
		description string
		priority    PriorityType
	}{
		{
			description: "less",
			priority:    NewestType,
		},
		{
			description: "tie breaker prioritizing newer messages",
			priority:    NewestType,
		},
		{
			description: "tie breaker prioritizing older messages",
			priority:    OldestType,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)
			pq := priorityQueue{}
			pq.queue = []item{oldestMsg, newestMsg, tieBreakerMsg}

			var err error
			pq.tieBreaker, err = priority(tc.priority)
			require.NoError(err)

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

func testTrim(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	msg0 := wrp.Message{
		Destination:      "mac:00deadbeef02/config",
		Payload:          []byte("{\"command\":\"GET\",\"names\":[]}"),
		QualityOfService: wrp.QOSMediumValue,
	}
	msg1 := wrp.Message{
		Destination:      "mac:00deadbeef02/config",
		Payload:          []byte("{\"command\":\"GET\",\"names\":[]}"),
		QualityOfService: wrp.QOSHighValue,
	}
	msg2 := wrp.Message{
		Destination:      "mac:00deadbeef02/config",
		Payload:          []byte("{\"command\":\"GET\",\"names\":[]}"),
		QualityOfService: wrp.QOSHighValue,
	}
	msg3 := wrp.Message{
		Destination:      "mac:00deadbeef03/config",
		Payload:          []byte("{\"command\":\"GET\",\"names\":[\"NoSuchParameter\"]}"),
		QualityOfService: wrp.QOSCriticalValue,
	}

	pq := priorityQueue{
		maxQueueBytes:   int64(len(msg0.Payload) + len(msg3.Payload)),
		maxMessageBytes: len(msg3.Payload),
		sizeBytes:       int64(len(msg0.Payload) + len(msg1.Payload)*2 + len(msg3.Payload)),
		priority:        NewestType,
	}
	pq.queue = []item{
		{
			msg:     &msg0,
			expires: time.Now().Add(DefaultCriticalExpires),
		},
		{
			msg:     &msg1,
			expires: time.Now(),
		},
		{
			msg:     &msg2,
			expires: time.Now(),
		},
		{
			msg:     &msg3,
			expires: time.Now().Add(DefaultCriticalExpires),
		},
	}

	var err error
	require.NoError(err)
	pq.trim()
	assert.Nil(pq.queue[1].msg.Payload)
	assert.Nil(pq.queue[2].msg.Payload)
	assert.NotEmpty(pq.queue[0].msg.Payload)
	assert.NotEmpty(pq.queue[3].msg.Payload)
	assert.Equal(pq.maxQueueBytes, pq.sizeBytes)
	assert.Equal(*pq.queue[0].msg, msg0)
	assert.Equal(*pq.queue[2].msg, msg1)
	assert.Equal(*pq.queue[2].msg, msg2)
	assert.Equal(*pq.queue[3].msg, msg3)
}

func testSwap(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	msg0 := wrp.Message{
		Destination: "mac:00deadbeef00/config",
	}
	msg2 := wrp.Message{
		Destination: "mac:00deadbeef02/config",
	}
	pq := priorityQueue{}
	pq.queue = []item{
		{
			msg:     &msg0,
			expires: time.Now(),
		},
		{
			msg: &wrp.Message{
				Destination: "mac:00deadbeef01/config",
			},
			expires: time.Now(),
		},
		{
			msg:     &msg2,
			expires: time.Now(),
		},
	}

	var err error
	pq.tieBreaker, err = priority(NewestType)
	require.NoError(err)
	pq.Swap(0, 2)
	// pq.queue[0] should contain msg2
	assert.Equal(msg2, *pq.queue[0].msg)
	// pq.queue[2] should contain msg0
	assert.Equal(msg0, *pq.queue[2].msg)
}

func testPush(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
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
	pq := priorityQueue{}

	var err error
	pq.tieBreaker, err = priority(NewestType)
	require.NoError(err)

	for _, msg := range messages {
		pq.Push(msg)
		assert.Equal(msg, *pq.queue[pq.Len()-1].msg)
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
					msg: &msg0,
				},
			},
			expectedMessage: msg0,
		},
		{
			description: "multiple messages with memory leak check",
			items: []item{
				{
					msg: &msg0,
				},
				{
					msg: &msg1,
				},
				{
					msg: &msg2,
				},
			},
			expectedMessage: msg2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			pq := priorityQueue{}
			pq.queue = tc.items

			var err error
			pq.tieBreaker, err = priority(NewestType)
			require.NoError(err)

			// no sorting is applied, Pop will pop the last message from priorityQueue's queue
			switch itm := pq.Pop().(type) {
			case nil:
				assert.Len(tc.items, 0)
			case item:
				assert.Equal(tc.expectedMessage, *itm.msg)
				require.NotEmpty(tc.items, "Pop() should have returned a nil instead of a wrp.Message")
				// check for memory leak
				assert.Empty(tc.items[len(tc.items)-1])
			default:
				require.Fail("Pop() returned an unknown type")
			}
		})
	}
}
