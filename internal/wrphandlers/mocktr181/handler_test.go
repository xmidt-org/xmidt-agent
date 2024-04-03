// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package mocktr181

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
	"go.uber.org/zap"
)

func TestHandler_HandleWrp(t *testing.T) {
	tests := []struct {
		description     string
		nextResult      error
		nextCallCount   int
		egressResult    error
		egressCallCount int
		msg             wrp.Message
		expectedErr     error
		validate        func(*assert.Assertions, wrp.Message) error
	}{
		{
			description:     "get success with multiple results",
			egressCallCount: 1,
			expectedErr:     nil,
			msg: wrp.Message{
				Type:        wrp.SimpleEventMessageType,
				Source:      "dns:tr1d1um.example.com/service/ignored",
				Destination: "event:event_1/ignored",
				Payload:     []byte("{\"command\":\"GET\",\"names\":[\"Device.DeviceInfo.\"]}"),
			},
			validate: func(a *assert.Assertions, msg wrp.Message) error {
				a.Equal(int64(http.StatusOK), *msg.Status)
				var result []Parameters
				err := json.Unmarshal(msg.Payload, &result)
				a.NoError(err)
				a.Equal(102, len(result))
				return nil
			},
		}, {
			description:     "set, success",
			egressCallCount: 1,
			msg: wrp.Message{
				Type:        wrp.SimpleEventMessageType,
				Source:      "dns:tr1d1um.example.com/service/ignored",
				Destination: "event:event_1/ignored",
				Payload:     []byte("{\"command\":\"SET\",\"parameters\":[{\"name\":\"Device.WiFi.Radio.10000.Name\",\"dataType\":0,\"value\":\"anothername\",\"attributes\":{\"notify\":0}}]}"),
			},
			validate: func(a *assert.Assertions, msg wrp.Message) error {
				a.Equal(int64(http.StatusAccepted), *msg.Status)

				return nil
			},
		}, {
			description:     "set, read only",
			egressCallCount: 1,
			msg: wrp.Message{
				Type:        wrp.SimpleEventMessageType,
				Source:      "dns:tr1d1um.example.com/service/ignored",
				Destination: "event:event_1/ignored",
				Payload:     []byte("{\"command\":\"SET\",\"parameters\":[{\"name\":\"Device.Bridging.MaxBridgeEntries\",\"dataType\":0,\"value\":\"anothername\",\"attributes\":{\"notify\":0}}]}"),
			},
			validate: func(a *assert.Assertions, msg wrp.Message) error {
				a.Equal(int64(http.StatusAccepted), *msg.Status)

				return nil
			},
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
			egress := wrpkit.HandlerFunc(func(msg wrp.Message) error {
				egressCallCount++
				if tc.validate != nil {
					assert.NoError(tc.validate(assert, msg))
				}
				return tc.egressResult
			})

			mockDefaults := []Option{
				FilePath("mock_tr181_test.json"),
			}

			h, err := New(next, egress, "some-source", zap.NewExample(), mockDefaults...)
			require.NoError(err)

			err = h.HandleWrp(tc.msg)
			assert.ErrorIs(err, tc.expectedErr)

			assert.Equal(tc.nextCallCount, nextCallCount)
			assert.Equal(tc.egressCallCount, egressCallCount)
		})
	}
}
