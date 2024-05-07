// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"testing"

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
		maxQueueSize            int
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
			maxQueueSize:      len(smallLowQOSMsg.Payload),
			maxMessageBytes:   len(smallLowQOSMsg.Payload),
			expectedQueueSize: 5,
		},
		{
			description:       "message too large with an empty queue",
			messages:          []wrp.Message{largeCriticalQOSMsg},
			maxQueueSize:      len(largeCriticalQOSMsg.Payload),
			maxMessageBytes:   len(largeCriticalQOSMsg.Payload) - 1,
			expectedQueueSize: 0,
		},
		{
			description:       "message too large with a nonempty queue",
			messages:          []wrp.Message{largeCriticalQOSMsg, largeCriticalQOSMsg},
			maxQueueSize:      len(largeCriticalQOSMsg.Payload),
			maxMessageBytes:   len(largeCriticalQOSMsg.Payload),
			expectedQueueSize: 1,
		},
		{
			description:       "remove some low priority messages to fit a higher priority message",
			messages:          []wrp.Message{mediumMediumQosMsg, mediumMediumQosMsg, mediumMediumQosMsg, largeCriticalQOSMsg},
			maxQueueSize:      len(mediumMediumQosMsg.Payload) * 3,
			maxMessageBytes:   len(largeCriticalQOSMsg.Payload),
			expectedQueueSize: 2,
		},
		{
			description:       "remove all low priority messages to fit a higher priority message",
			messages:          []wrp.Message{smallLowQOSMsg, smallLowQOSMsg, smallLowQOSMsg, smallLowQOSMsg, largeCriticalQOSMsg},
			maxQueueSize:      len(largeCriticalQOSMsg.Payload),
			maxMessageBytes:   len(largeCriticalQOSMsg.Payload),
			expectedQueueSize: 1,
		},
		{
			description:             "drop incoming low priority messages since queue is filled with higher priority message",
			messages:                []wrp.Message{largeCriticalQOSMsg, largeCriticalQOSMsg, smallLowQOSMsg, mediumMediumQosMsg},
			maxQueueSize:            len(largeCriticalQOSMsg.Payload) * 2,
			maxMessageBytes:         len(largeCriticalQOSMsg.Payload),
			expectedQueueSize:       2,
			expectedDequeueSequence: []wrp.Message{largeCriticalQOSMsg, largeCriticalQOSMsg},
		},
		{
			description:             "dequeue all messages from highest to lowest priority",
			messages:                enqueueSequenceTest,
			maxQueueSize:            queueSizeSequenceTest,
			maxMessageBytes:         len(largeCriticalQOSMsg.Payload),
			expectedQueueSize:       len(enqueueSequenceTest),
			expectedDequeueSequence: dequeueSequenceTest,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)
			pq := priorityQueue{maxQueueSize: tc.maxQueueSize, maxMessageBytes: tc.maxMessageBytes}
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
	pq := priorityQueue{}

	assert.Equal(0, pq.size)
	pq.Push(msg)
	pq.Push(msg)
	assert.Equal(len(msg.Payload)*2, pq.size)
}
func testLen(t *testing.T) {
	assert := assert.New(t)
	pq := priorityQueue{queue: []wrp.Message{
		{
			Destination: "mac:00deadbeef00/config",
		},
		{
			Destination: "mac:00deadbeef01/config",
		},
	}}

	assert.Equal(len(pq.queue), pq.Len())
}

func testLess(t *testing.T) {
	assert := assert.New(t)
	pq := priorityQueue{queue: []wrp.Message{
		{
			Destination:      "mac:00deadbeef00/config",
			QualityOfService: wrp.QOSCriticalValue,
		},
		{
			Destination:      "mac:00deadbeef01/config",
			QualityOfService: wrp.QOSLowValue,
		},
	}}

	// wrp.QOSCriticalValue > wrp.QOSLowValue
	assert.True(pq.Less(0, 1))
	// wrp.QOSLowValue > wrp.QOSCriticalValue
	assert.False(pq.Less(1, 0))
}

func testSwap(t *testing.T) {
	assert := assert.New(t)
	msg0 := wrp.Message{
		Destination: "mac:00deadbeef00/config",
	}
	msg2 := wrp.Message{
		Destination: "mac:00deadbeef02/config",
	}
	pq := priorityQueue{queue: []wrp.Message{
		msg0,
		{
			Destination: "mac:00deadbeef01/config",
		},
		msg2,
	}}

	pq.Swap(0, 2)
	// pq.queue[0] should contain msg2
	assert.Equal(msg2, pq.queue[0])
	// pq.queue[2] should contain msg0
	assert.Equal(msg0, pq.queue[2])
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
	pq := priorityQueue{}
	for _, msg := range messages {
		pq.Push(msg)
	}

	assert.Equal(messages, pq.queue)
}

func testPop(t *testing.T) {
	tests := []struct {
		description     string
		messages        []wrp.Message
		expectedMessage wrp.Message
	}{
		// success cases
		{
			description: "empty queue",
		},
		{
			description: "single message with memory leak check",
			messages: []wrp.Message{
				{
					Destination: "mac:00deadbeef00/config",
				},
			},
			expectedMessage: wrp.Message{
				Destination: "mac:00deadbeef00/config",
			},
		},
		{
			description: "multiple messages with memory leak check",
			messages: []wrp.Message{
				{
					Destination: "mac:00deadbeef00/config",
				},
				{
					Destination: "mac:00deadbeef01/config",
				},
				{
					Destination: "mac:00deadbeef02/config",
				},
			},
			expectedMessage: wrp.Message{
				Destination: "mac:00deadbeef02/config",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			pq := priorityQueue{queue: tc.messages}
			// no sorting is applied, Pop will pop the last message from priorityQueue's queue
			switch msg := pq.Pop().(type) {
			case nil:
				assert.Len(tc.messages, 0)
			case wrp.Message:
				assert.Equal(tc.expectedMessage, msg)
				require.NotEmpty(tc.messages, "Pop() should have returned a nil instead of a wrp.Message")
				// check for memory leak
				assert.Equal(wrp.Message{}, tc.messages[len(tc.messages)-1])
			default:
				require.Fail("Pop() returned an unknown type")
			}
		})
	}
}
