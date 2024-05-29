// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/wrphandlers/qos"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

func TestHandler_HandleWrp(t *testing.T) {
	var (
		nextCallCount atomic.Int64
		msg           = wrp.Message{
			Type:             wrp.SimpleRequestResponseMessageType,
			Source:           "dns:tr1d1um.example.com/service/ignored",
			Destination:      "mac:00deadbeef00/config",
			Payload:          []byte("{\"command\":\"GET\",\"names\":[\"NoSuchParameter\"]}"),
			QualityOfService: wrp.QOSLowValue,
		}
	)

	tests := []struct {
		description     string
		maxQueueBytes   int
		maxMessageBytes int
		// int64 required for nextCallCount atomic.Int64 comparison
		nextCallCount        int64
		next                 wrpkit.Handler
		shutdown             bool
		failDeliveryOnce     bool
		shouldHalt           bool
		expectedNewErr       error
		expectedHandleWRPErr error
	}{
		// success cases
		{
			description:     "enqueued and delivered message",
			maxQueueBytes:   100,
			maxMessageBytes: 50,
			nextCallCount:   1,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
		},
		{
			description:     "re-enqueue message that failed its delivery",
			maxQueueBytes:   100,
			maxMessageBytes: 50,
			nextCallCount:   2,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)
				if nextCallCount.Load() < 2 {
					// message should be re-enqueue and re-delivered
					return errors.New("random error")
				}

				return nil
			}),
			failDeliveryOnce: true,
		},
		{
			description:     "queue messages while message delivery is blocked",
			maxQueueBytes:   100,
			maxMessageBytes: 50,
			nextCallCount:   0,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				// halt qos message delivery
				time.Sleep(1 * time.Second)

				return nil
			}),
			shouldHalt: true,
		},
		{
			description:     "zero MaxQueueBytes option value",
			maxQueueBytes:   0,
			maxMessageBytes: 50,
			nextCallCount:   1,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
		},
		{
			description:     "zero MaxMessageBytes option value",
			maxQueueBytes:   qos.DefaultMaxQueueBytes,
			maxMessageBytes: 0,
			nextCallCount:   1,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
		},
		// failure cases
		{
			description:     "invalid inputs for qos.New",
			maxQueueBytes:   100,
			maxMessageBytes: 50,
			expectedNewErr:  qos.ErrInvalidInput,
		},
		{
			description:     "negative MaxQueueBytes option value",
			maxQueueBytes:   -1,
			maxMessageBytes: 50,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
			expectedNewErr: qos.ErrMisconfiguredQOS,
		},
		{
			description:     "negative MaxMessageBytes option value",
			maxQueueBytes:   100,
			maxMessageBytes: -1,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
			expectedNewErr: qos.ErrMisconfiguredQOS,
		},
		{
			description:     "qos has stopped",
			maxQueueBytes:   100,
			maxMessageBytes: 50,
			nextCallCount:   0,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
			shutdown:             true,
			expectedHandleWRPErr: qos.ErrQOSHasShutdown,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			h, err := qos.New(tc.next, qos.MaxQueueBytes(int64(tc.maxQueueBytes)), qos.MaxMessageBytes(tc.maxMessageBytes), qos.PrioritizeOldest(false))
			if tc.expectedNewErr != nil {
				assert.ErrorIs(err, tc.expectedNewErr)
				assert.Nil(h)

				return
			} else {
				require.NoError(err)
				require.NotNil(h)
			}

			h.Start()
			// Allow multiple calls to Start.
			h.Start()

			if tc.shutdown {
				h.Stop()
				// Allow multiple calls to Stop.
				h.Stop()

				// allow qos ingestion to stop beforing sending a message
				time.Sleep(10 * time.Millisecond)
			} else {
				defer h.Stop()
			}

			err = h.HandleWrp(msg)
			if tc.expectedHandleWRPErr != nil {
				assert.ErrorIs(err, tc.expectedHandleWRPErr)
			} else {
				assert.Nil(err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			for {
				if nextCallCount.Load() > 0 || tc.shutdown {
					break
				}

				if ctx.Err() != nil {
					if tc.shouldHalt {
						// send another msg to verify message delivery is blocked
						// but qos is still enqueueing messages
						assert.Nil(h.HandleWrp(msg))
						break
					}
					assert.Fail("timed out waiting for messages")
					return
				}

				time.Sleep(10 * time.Millisecond)
			}

			// Sleep required by the test "re-enqueue message that failed its delivery"
			// in order to wait for any re-enqueue messages to be re-delivered
			time.Sleep(100 * time.Millisecond)
			assert.Equal(tc.nextCallCount, nextCallCount.Load())
			nextCallCount.Swap(0)
		})
	}
}
