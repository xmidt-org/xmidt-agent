// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package missing_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/wrp-go/v4"
	"github.com/xmidt-org/xmidt-agent/internal/wrphandlers/missing"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

func TestHandler_HandleWrp(t *testing.T) {
	randomErr := errors.New("random error")

	tests := []struct {
		description     string
		nextResult      error
		nextCallCount   int
		egressResult    error
		egressCallCount int
		msg             wrp.Message
		expectedErr     error
		validate        func(wrp.Message) error
	}{
		{
			description:   "normal message",
			nextCallCount: 1,
			msg: wrp.Message{
				Type:        wrp.SimpleEventMessageType,
				Source:      "dns:tr1d1um.example.com/service/ignored",
				Destination: "event:event_1/ignored",
			},
		}, {
			description:   "error with an msg that doesn't require a response (random error)",
			nextCallCount: 1,
			nextResult:    randomErr,
			msg: wrp.Message{
				Type:        wrp.SimpleEventMessageType,
				Source:      "dns:tr1d1um.example.com/service/ignored",
				Destination: "event:event_1/ignored",
			},
			expectedErr: randomErr,
		}, {
			description:   "error with an msg that doesn't require a response (no handler)",
			nextCallCount: 1,
			nextResult:    wrpkit.ErrNotHandled,
			msg: wrp.Message{
				Type:        wrp.SimpleEventMessageType,
				Source:      "dns:tr1d1um.example.com/service/ignored",
				Destination: "event:event_1/ignored",
			},
			expectedErr: wrpkit.ErrNotHandled,
		}, {
			description:   "error with an msg that requires a response, but was handled",
			nextCallCount: 1,
			nextResult:    randomErr,
			msg: wrp.Message{
				Type:        wrp.SimpleRequestResponseMessageType,
				Source:      "dns:tr1d1um.example.com/service/ignored",
				Destination: "mac:112233445566/some-service",
			},
			expectedErr: randomErr,
		}, {
			description:     "unhandled message, but requires a response",
			nextCallCount:   1,
			nextResult:      wrpkit.ErrNotHandled,
			egressCallCount: 1,
			msg: wrp.Message{
				Type:        wrp.SimpleRequestResponseMessageType,
				Source:      "dns:tr1d1um.example.com/service/ignored",
				Destination: "mac:112233445566/some-service",
			},
		}, {
			description:     "unhandled message, requires a response, error sending response",
			nextCallCount:   1,
			nextResult:      wrpkit.ErrNotHandled,
			egressCallCount: 1,
			egressResult:    randomErr,
			msg: wrp.Message{
				Type:        wrp.SimpleRequestResponseMessageType,
				Source:      "dns:tr1d1um.example.com/service/ignored",
				Destination: "mac:112233445566/some-service",
			},
			expectedErr: randomErr,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			nextCallCount := 0
			next := wrpkit.HandlerFunc(func(wrp.Message) error {
				nextCallCount++
				return tc.nextResult
			})

			egressCallCount := 0
			egress := wrpkit.HandlerFunc(func(wrp.Message) error {
				egressCallCount++
				if tc.validate != nil {
					assert.NoError(tc.validate(tc.msg))
				}
				return tc.egressResult
			})

			h, err := missing.New(next, egress, "self:/xmidt-agent/missing")
			require.NoError(err)

			err = h.HandleWrp(tc.msg)
			assert.ErrorIs(err, tc.expectedErr)

			assert.Equal(tc.nextCallCount, nextCallCount)
			assert.Equal(tc.egressCallCount, egressCallCount)
		})
	}
}
