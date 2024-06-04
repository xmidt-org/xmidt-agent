// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos_test

import (
	"context"
	"errors"
	"math"
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
		description string
		options     []qos.Option
		priority    qos.PriorityType
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
			description:   "enqueued and delivered message prioritizing newer messages",
			options:       []qos.Option{qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(qos.NewestType)},
			nextCallCount: 1,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
		},
		{
			description:   "enqueued and delivered message prioritizing older messages",
			options:       []qos.Option{qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(qos.OldestType)},
			nextCallCount: 1,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
		},
		{
			description:   "re-enqueue message that failed its delivery",
			options:       []qos.Option{qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(qos.NewestType)},
			nextCallCount: 2,
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
			description: "queue messages while message delivery is blocked",
			options:     []qos.Option{qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(qos.NewestType)},

			nextCallCount: 0,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				// halt qos message delivery
				time.Sleep(1 * time.Second)

				return nil
			}),
			shouldHalt: true,
		},
		{
			description: "zero MaxQueueBytes option value",
			options:     []qos.Option{qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(qos.NewestType)},

			nextCallCount: 1,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
		},
		{
			description:   "zero MaxMessageBytes option value",
			options:       []qos.Option{qos.MaxQueueBytes(int64(0)), qos.MaxMessageBytes(0), qos.Priority(qos.NewestType)},
			nextCallCount: 1,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
		},
		{
			description:   "non-negative LowQOSExpires option value",
			options:       []qos.Option{qos.LowQOSExpires(0), qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(qos.NewestType)},
			nextCallCount: 1,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
		},
		{
			description:   "non-negative MediumQOSExpires option value",
			options:       []qos.Option{qos.MediumQOSExpires(0), qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(qos.NewestType)},
			nextCallCount: 1,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
		},
		{
			description:   "non-negative HighQOSExpires option value",
			options:       []qos.Option{qos.HighQOSExpires(0), qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(qos.NewestType)},
			nextCallCount: 1,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
		},
		{
			description:   "non-negative CriticalQOSExpires option value",
			options:       []qos.Option{qos.CriticalQOSExpires(0), qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(qos.NewestType)},
			nextCallCount: 1,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
		},
		// failure cases
		{
			description:    "invalid inputs for qos.New",
			options:        []qos.Option{qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(qos.NewestType)},
			expectedNewErr: qos.ErrInvalidInput,
		},
		{
			description: "negative MaxQueueBytes option value",
			options:     []qos.Option{qos.MaxQueueBytes(int64(-1)), qos.MaxMessageBytes(50), qos.Priority(qos.NewestType)},
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
			expectedNewErr: qos.ErrMisconfiguredQOS,
		},
		{
			description: "negative MaxMessageBytes option value",
			options:     []qos.Option{qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(-1), qos.Priority(qos.NewestType)},
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
			expectedNewErr: qos.ErrMisconfiguredQOS,
		},
		{
			description: "negative invalid priority type option value",
			options:     []qos.Option{qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(-1)},
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
			expectedNewErr: qos.ErrMisconfiguredQOS,
		},
		{
			description: "positive invalid priority type option value",
			options:     []qos.Option{qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(math.MaxInt64)},
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
			expectedNewErr: qos.ErrMisconfiguredQOS,
		},
		{
			description: "unknown priority type option value",
			options:     []qos.Option{qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(qos.UnknownType)},
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
			expectedNewErr: qos.ErrMisconfiguredQOS,
		},
		{
			description:   "qos has stopped",
			options:       []qos.Option{qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(qos.NewestType)},
			nextCallCount: 0,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
			shutdown:             true,
			expectedHandleWRPErr: qos.ErrQOSHasShutdown,
		},
		{
			description:   "negative LowQOSExpires option value",
			options:       []qos.Option{qos.LowQOSExpires(-1), qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(qos.NewestType)},
			nextCallCount: 0,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
			expectedNewErr: qos.ErrMisconfiguredQOS,
		},
		{
			description:   "negative MediumQOSExpires option value",
			options:       []qos.Option{qos.MediumQOSExpires(-1), qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(qos.NewestType)},
			nextCallCount: 0,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
			expectedNewErr: qos.ErrMisconfiguredQOS,
		},
		{
			description:   "negative HighQOSExpires option value",
			options:       []qos.Option{qos.HighQOSExpires(-1), qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(qos.NewestType)},
			nextCallCount: 0,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
			expectedNewErr: qos.ErrMisconfiguredQOS,
		},
		{
			description:   "negative CriticalQOSExpires option value",
			options:       []qos.Option{qos.CriticalQOSExpires(-1), qos.MaxQueueBytes(int64(100)), qos.MaxMessageBytes(50), qos.Priority(qos.NewestType)},
			nextCallCount: 0,
			next: wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount.Add(1)

				return nil
			}),
			expectedNewErr: qos.ErrMisconfiguredQOS,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			h, err := qos.New(tc.next, tc.options...)
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
