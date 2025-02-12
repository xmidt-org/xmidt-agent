// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/xmidt-org/wrp-go/v4"
)

type mockNetworkService struct {
	mock.Mock
}

func newMockNetworkService() *mockNetworkService { return &mockNetworkService{} }

func (m *mockNetworkService) GetInterfaces() ([]net.Interface, error) {
	args := m.Called()
	return args.Get(0).([]net.Interface), args.Error(1)
}

func (m *mockNetworkService) GetInterfaceNames() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

type ConveySuite struct {
	suite.Suite
	conveyHeaderProvider *MetadataProvider
	mockNetworkService   *mockNetworkService
}

func (suite *ConveySuite) SetupTest() {
	mockNetworkService := newMockNetworkService()
	suite.mockNetworkService = mockNetworkService

	opts := []Option{
		NetworkServiceOpt(mockNetworkService),
		FieldsOpt([]string{"fw-name", "hw-model", "hw-manufacturer", "hw-serial-number", "hw-last-reboot-reason", "webpa-protocol", "boot-time", "boot-time-retry-wait", "webpa-interface-used", "interfaces-available"}),
		SerialNumberOpt("123"),
		HardwareModelOpt("some-model"),
		ManufacturerOpt("some-manufacturer"),
		FirmwareOpt("1.1"),
		LastRebootReasonOpt("some-reason"),
		XmidtProtocolOpt("some-protocol"),
		BootTimeOpt("1111111111"),
		BootRetryWaitOpt(time.Second),
		InterfaceUsedOpt("erouter0"),
	}

	conveyHeaderProvider, err := New(opts...)
	suite.NoError(err)

	suite.conveyHeaderProvider = conveyHeaderProvider
	suite.mockNetworkService = mockNetworkService
}

func TestConveySuite(t *testing.T) {
	suite.Run(t, new(ConveySuite))
}

func (suite *ConveySuite) TestGetConveyHeader() {
	suite.mockNetworkService.On("GetInterfaceNames").Return([]string{"erouter0", "eth0"}, nil)
	header := suite.conveyHeaderProvider.GetMetadata()

	suite.Equal("1.1", header["fw-name"])
	suite.Equal("some-model", header["hw-model"])
	suite.Equal("some-manufacturer", header["hw-manufacturer"])
	suite.Equal("123", header["hw-serial-number"])
	suite.Equal("some-reason", header["hw-last-reboot-reason"])
	suite.Equal("some-protocol", header["webpa-protocol"])
	suite.Equal("1111111111", header["boot-time"])
	suite.Equal("1", header["boot-time-retry-wait"])
	suite.Equal("erouter0,eth0", header["interfaces-available"])
	suite.Equal("erouter0", header["webpa-interface-used"])
}

func (suite *ConveySuite) TestGetConveyHeaderSubsetFields() {
	suite.mockNetworkService.On("GetInterfaceNames").Return([]string{"docsis"}, nil)
	suite.conveyHeaderProvider.fields = []string{"fw-name", "hw-model"}

	header := suite.conveyHeaderProvider.GetMetadata()

	suite.Equal("1.1", header["fw-name"])
	suite.Equal("some-model", header["hw-model"])
	suite.Nil(header["hw-manufacturer"])
	suite.Nil(header["hw-serial-number"])
	suite.Nil(header["hw-last-reboot-reason"])
	suite.Nil(header["webpa-protocol"])
	suite.Nil(header["boot-time"])
	suite.Nil(header["boot-time-retry-wait"])
	suite.Nil(header["interfaces-available"])
	suite.Nil(header["webpa-interface-used"])
}

func (suite *ConveySuite) TestDecorate() {
	suite.mockNetworkService.On("GetInterfaceNames").Return([]string{"docsis"}, nil)

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	suite.NoError(err)
	suite.NotNil(req)

	err = suite.conveyHeaderProvider.Decorate(req.Header)
	suite.NoError(err)

	suite.NotNil(req.Header.Get(HeaderName))
}

func (suite *ConveySuite) TestDecorateMsg() {
	suite.mockNetworkService.On("GetInterfaceNames").Return([]string{"erouter0"}, nil)

	msg := new(wrp.Message)

	err := suite.conveyHeaderProvider.DecorateMsg(msg)
	suite.NoError(err)

	suite.Equal("erouter0", msg.Metadata["interfaces-available"])
}
