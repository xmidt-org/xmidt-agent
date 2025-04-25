// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0
package cloud

import (
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/event"
)

type MockCloudHandler struct {
	mock.Mock
}

func NewMockCloudHandler() *MockCloudHandler { return &MockCloudHandler{} }

func (m *MockCloudHandler) Start() {
	m.Called()
}

func (m *MockCloudHandler) Stop() {
	m.Called()
}

func (m *MockCloudHandler) Name() string {
	args := m.Called()
	return args.Get(0).(string)
}

// func (m *MockCloudHandler) Send(ctx context.Context, msg wrp.Message) error {
// 	args := m.Called(ctx, msg)
// 	return args.Error(0)
// }

func (m *MockCloudHandler) AddMessageListener(l event.MsgListener) event.CancelFunc {
	args := m.Called(l)
	return args.Get(0).(event.CancelFunc)
}

func (m *MockCloudHandler) AddConnectListener(l event.ConnectListener) event.CancelFunc {
	args := m.Called(l)
	return args.Get(0).(event.CancelFunc)
}

func (m *MockCloudHandler) HandleWrp(msg wrp.Message) error {
	args := m.Called(msg)
	return args.Error(0)
}
