// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/wrphandlers/auth"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

func TestHandler_HandleWrp(t *testing.T) {
	//randomErr := errors.New("random error")

	tests := []struct {
		description     string
		nextResult      error
		nextCallCount   int
		egressResult    error
		egressCallCount int
		partner         string
		partners        []string
		msg             wrp.Message
		expectedErr     error
		validate        func(wrp.Message) error
	}{
		{
			description:   "normal message, good auth",
			nextCallCount: 1,
			msg: wrp.Message{
				Type:        wrp.SimpleEventMessageType,
				Source:      "dns:tr1d1um.example.com/service/ignored",
				Destination: "event:event_1/ignored",
				PartnerIDs:  []string{"example-partner"},
			},
			partner: "example-partner",
		}, {
			description:   "normal message, wildcard auth",
			nextCallCount: 1,
			msg: wrp.Message{
				Type:        wrp.SimpleEventMessageType,
				Source:      "dns:tr1d1um.example.com/service/ignored",
				Destination: "event:event_1/ignored",
				PartnerIDs:  []string{"example-partner"},
			},
			partner: "*",
		}, {
			description: "partner not allowed, no response needed",
			msg: wrp.Message{
				Type:        wrp.SimpleEventMessageType,
				Source:      "dns:tr1d1um.example.com/service/ignored",
				Destination: "event:event_1/ignored",
				PartnerIDs:  []string{"example-partner"},
			},
			partner:     "some-other-partner",
			expectedErr: auth.ErrUnauthorized,
		}, {
			egressCallCount: 1,
			description:     "partner not allowed, response needed",
			msg: wrp.Message{
				Type:            wrp.SimpleRequestResponseMessageType,
				Source:          "dns:tr1d1um.example.com/service/ignored",
				Destination:     "mac:112233445566/service",
				TransactionUUID: "1234",
				PartnerIDs:      []string{"example-partner"},
			},
			partner:     "some-other-partner",
			expectedErr: auth.ErrUnauthorized,
		}, {
			egressCallCount: 1,
			description:     "no partner provided, response needed",
			msg: wrp.Message{
				Type:            wrp.SimpleRequestResponseMessageType,
				Source:          "dns:tr1d1um.example.com/service/ignored",
				Destination:     "mac:112233445566/service",
				TransactionUUID: "1234",
			},
			partner:     "some-partner",
			expectedErr: auth.ErrUnauthorized,
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

			partners := append(tc.partners, tc.partner)

			h, err := auth.New(next, egress, "self:/xmidt-agent/missing", partners...)
			require.NoError(err)

			err = h.HandleWrp(tc.msg)
			assert.ErrorIs(err, tc.expectedErr)

			assert.Equal(tc.nextCallCount, nextCallCount)
			assert.Equal(tc.egressCallCount, egressCallCount)
		})
	}
}
