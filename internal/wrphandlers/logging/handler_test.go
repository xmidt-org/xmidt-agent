// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package loghandler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/wrp-go/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type MockHandler struct {
	mock.Mock
}

func (m *MockHandler) HandleWrp(msg wrp.Message) error {
	args := m.Called(msg)
	return args.Error(0)
}

func TestNew(t *testing.T) {
	logger := zap.NewNop()
	next := &MockHandler{}

	handler, err := New(next, logger)
	assert.NoError(t, err)
	assert.NotNil(t, handler)
	assert.Equal(t, zapcore.DebugLevel, handler.level)

	handler, err = New(nil, logger)
	assert.Error(t, err)
	assert.Nil(t, handler)

	handler, err = New(next, nil)
	assert.Error(t, err)
	assert.Nil(t, handler)
}

func TestHandleWrp(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	next := &MockHandler{}
	handler, err := New(next, logger)
	assert.NoError(t, err)
	assert.NotNil(t, handler)

	msg := wrp.Message{
		Type:            wrp.SimpleRequestResponseMessageType,
		Source:          "source",
		Destination:     "destination",
		Payload:         []byte("payload"),
		TransactionUUID: "uuid",
	}

	next.On("HandleWrp", msg).Return(nil)

	err = handler.HandleWrp(msg)
	assert.NoError(t, err)
	next.AssertExpectations(t)
}
