// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package xmidt_agent_crud

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/wrp-go/v4"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

type mockLogLevel struct {
	mock.Mock
}

func newMockLogLevel() *mockLogLevel { return &mockLogLevel{} }

func (m *mockLogLevel) SetLevel(level string, d time.Duration) error {
	args := m.Called(level, d)
	return args.Error(0)
}

func TestHandler_HandleWrp(t *testing.T) {
	tests := []struct {
		description     string
		egressResult    error
		egressCallCount int
		msg             wrp.Message
		expectedErr     error
		logLevelMock    *mockLogLevel
		mockCalls       func(*mockLogLevel)
		validate        func(*assert.Assertions, wrp.Message, *mockLogLevel) error
	}{
		{
			description:     "set log level with duration",
			egressCallCount: 1,
			expectedErr:     nil,
			msg: wrp.Message{
				Type:        wrp.UpdateMessageType,
				Source:      "dns:tr1d1um.example.com/service/ignored",
				Destination: "xmidt-agent",
				Path:        "loglevel",
				Payload:     []byte("{\"loglevel\":\"debug\",\"duration\":\"1m\"}"),
			},
			logLevelMock: newMockLogLevel(),
			mockCalls: func(logLevelMock *mockLogLevel) {
				logLevelMock.On("SetLevel", "debug", 1*time.Minute).Return(nil)
			},
			validate: func(a *assert.Assertions, msg wrp.Message, logLevelMock *mockLogLevel) error {
				a.Equal(int64(http.StatusOK), *msg.Status)
				logLevelMock.AssertCalled(t, "SetLevel", "debug", 1*time.Minute)
				return nil
			},
		},
		{
			description:     "set log level without duration",
			egressCallCount: 1,
			expectedErr:     nil,
			msg: wrp.Message{
				Type:        wrp.UpdateMessageType,
				Source:      "dns:tr1d1um.example.com/service/ignored",
				Destination: "xmidt-agent",
				Path:        "loglevel",
				Payload:     []byte("{\"loglevel\":\"debug\"}"),
			},
			logLevelMock: newMockLogLevel(),
			mockCalls: func(logLevelMock *mockLogLevel) {
				logLevelMock.On("SetLevel", "debug", 30*time.Minute).Return(nil)
			},
			validate: func(a *assert.Assertions, msg wrp.Message, logLevelMock *mockLogLevel) error {
				a.Equal(int64(http.StatusOK), *msg.Status)
				logLevelMock.AssertCalled(t, "SetLevel", "debug", 30*time.Minute)
				return nil
			},
		},
		{
			description:     "set log level with a bad duration",
			egressCallCount: 1,
			expectedErr:     nil,
			msg: wrp.Message{
				Type:        wrp.UpdateMessageType,
				Source:      "dns:tr1d1um.example.com/service/ignored",
				Destination: "xmidt-agent",
				Path:        "loglevel",
				Payload:     []byte("{\"loglevel\":\"debug\",\"duration\":\"1zzzzzz\"}"),
			},
			logLevelMock: newMockLogLevel(),
			mockCalls: func(logLevelMock *mockLogLevel) {
				logLevelMock.On("SetLevel", "debug", 30*time.Minute).Return(nil)
			},
			validate: func(a *assert.Assertions, msg wrp.Message, logLevelMock *mockLogLevel) error {
				a.Equal(int64(http.StatusOK), *msg.Status)
				logLevelMock.AssertCalled(t, "SetLevel", "debug", 30*time.Minute)
				return nil
			},
		},
		{
			description:     "send some nonexistent action",
			egressCallCount: 1,
			expectedErr:     nil,
			msg: wrp.Message{
				Type:        wrp.UpdateMessageType,
				Source:      "dns:tr1d1um.example.com/service/ignored",
				Destination: "xmidt-agent",
				Path:        "no_such_path",
				Payload:     []byte("{\"loglevel\":\"debug\"}"),
			},
			logLevelMock: newMockLogLevel(),
			mockCalls: func(logLevelMock *mockLogLevel) {

			},
			validate: func(a *assert.Assertions, msg wrp.Message, logLevelMock *mockLogLevel) error {
				a.Equal(int64(http.StatusBadRequest), *msg.Status)
				logLevelMock.AssertNotCalled(t, "SetLevel")
				return nil
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)
			var h *Handler

			egressCallCount := 0
			egress := wrpkit.HandlerFunc(func(msg wrp.Message) error {
				egressCallCount++
				if tc.validate != nil {
					assert.NoError(tc.validate(assert, msg, tc.logLevelMock))
				}
				return tc.egressResult
			})

			tc.mockCalls(tc.logLevelMock)

			h, err := New(egress, "some-source", tc.logLevelMock)
			require.NoError(err)

			err = h.HandleWrp(tc.msg)
			assert.ErrorIs(err, tc.expectedErr)

			assert.Equal(tc.egressCallCount, egressCallCount)
		})
	}
}
