// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package net

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type mockNetworkWrapper struct {
	mock.Mock
}

func newMockNetworkWrapper() *mockNetworkWrapper { return &mockNetworkWrapper{} }

func (m *mockNetworkWrapper) Interfaces() ([]net.Interface, error) {
	args := m.Called()
	return args.Get(0).([]net.Interface), args.Error(1)
}

type NetworkServiceSuite struct {
	suite.Suite
	networkService     NetworkServicer
	mockNetworkWrapper *mockNetworkWrapper
}

func TestNetworkServiceSuite(t *testing.T) {
	suite.Run(t, new(NetworkServiceSuite))
}

func (suite *NetworkServiceSuite) SetupTest() {
	mockNetworkWrapper := newMockNetworkWrapper()
	networkService := New(mockNetworkWrapper)
	suite.networkService = networkService
	suite.mockNetworkWrapper = mockNetworkWrapper
}

func (suite *NetworkServiceSuite) TestGetInterfaceNames() {
	iface := net.Interface{
		Name:  "erouter0",
		Flags: 32,
	}

	suite.mockNetworkWrapper.On("Interfaces").Return([]net.Interface{iface}, nil)
	names, err := suite.networkService.GetInterfaceNames()
	suite.NoError(err)
	suite.Equal(1, len(names))
	suite.Equal("erouter0", names[0])

}

func (suite *NetworkServiceSuite) TestGetInterfaceNamesNotRunning() {
	iface := net.Interface{
		Name:  "erouter0",
		Flags: 0,
	}

	suite.mockNetworkWrapper.On("Interfaces").Return([]net.Interface{iface}, nil)
	names, err := suite.networkService.GetInterfaceNames()
	suite.NoError(err)
	suite.Equal(0, len(names))
}

func (suite *NetworkServiceSuite) TestGetInterfaceNamesError() {
	suite.mockNetworkWrapper.On("Interfaces").Return([]net.Interface{}, errors.New("some network error"))
	names, err := suite.networkService.GetInterfaceNames()
	suite.Error(err)
	suite.Equal(0, len(names))
}
