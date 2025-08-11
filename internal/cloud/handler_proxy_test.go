// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/event"
)

var TestCancelFunc event.CancelFunc = func() {}

type ProxySuite struct {
	suite.Suite
	got            *Proxy
	mockQuicClient *MockCloudHandler
	mockWebsocket  *MockCloudHandler
}

func TestProxySuite(t *testing.T) {
	suite.Run(t, new(ProxySuite))
}

func (suite *ProxySuite) SetupTest() {
	mockQuicHandler := NewMockCloudHandler()
	mockWebsocketHandler := NewMockCloudHandler()

	mockQuicHandler.On("AddConnectListener", mock.Anything).Return(TestCancelFunc)
	mockWebsocketHandler.On("AddConnectListener", mock.Anything).Return(TestCancelFunc)
	mockQuicHandler.On("AddMessageListener", mock.Anything).Return(TestCancelFunc)
	mockWebsocketHandler.On("AddMessageListener", mock.Anything).Return(TestCancelFunc)
	mockQuicHandler.On("IsEnabled", mock.Anything).Return(true)
	mockWebsocketHandler.On("IsEnabled", mock.Anything).Return(true)

	got, err := New(
		QuicClient(mockQuicHandler),
		Websocket(mockWebsocketHandler),
		PreferQuic(true),
		MaxTries(2),
	)

	mockQuicHandler.AssertCalled(suite.T(), "AddConnectListener", mock.Anything)
	mockQuicHandler.AssertCalled(suite.T(), "AddMessageListener", mock.Anything)

	mockWebsocketHandler.AssertCalled(suite.T(), "AddConnectListener", mock.Anything)
	mockWebsocketHandler.AssertCalled(suite.T(), "AddMessageListener", mock.Anything)

	suite.NoError(err)
	suite.mockQuicClient = mockQuicHandler
	suite.mockWebsocket = mockWebsocketHandler
	suite.got = got.(*Proxy)
}

func (suite *ProxySuite) TestNew() {

	// quic preferred

	suite.Equal(suite.got.active, suite.mockQuicClient)
	suite.Equal(suite.got.activeWrpHandler, suite.mockQuicClient)
	suite.True(suite.got.IsEnabled())

	// websocket preferred

	h, err := New(
		QuicClient(suite.mockQuicClient),
		Websocket(suite.mockWebsocket),
		PreferQuic(false),
		MaxTries(2),
	)

	suite.NoError(err)
	p := h.(*Proxy)
	suite.Equal(p.active, suite.mockWebsocket)
	suite.Equal(p.activeWrpHandler, suite.mockWebsocket)
	suite.True(suite.got.IsEnabled())

	// missing quic client

	_, err = New(
		Websocket(suite.mockWebsocket),
		PreferQuic(false),
		MaxTries(2),
	)
	suite.Error(err)

	// nil quic client

	_, err = New(
		QuicClient(nil),
		Websocket(suite.mockWebsocket),
		PreferQuic(false),
		MaxTries(2),
	)
	suite.Error(err)

	// missing websocket

	_, err = New(
		QuicClient(suite.mockQuicClient),
		PreferQuic(false),
		MaxTries(2),
	)
	suite.Error(err)

	// nil websocket

	_, err = New(
		Websocket(nil),
		QuicClient(suite.mockQuicClient),
		PreferQuic(false),
		MaxTries(2),
	)
	suite.Error(err)
}

func (suite *ProxySuite) TestProxyMessageListener() {
	msgCount := 0

	// external resource can call this after New
	suite.got.AddMessageListener(
		event.MsgListenerFunc(
			func(m wrp.Message) {
				msgCount++
			}))

	suite.got.msgListeners.Visit(func(l event.MsgListener) {
		l.OnMessage(wrp.Message{})
	})
	suite.Equal(1, msgCount)
}

func (suite *ProxySuite) TestProxyConnectListener() {
	eCount := 0

	// external resource can call this after New
	suite.got.AddConnectListener(
		event.ConnectListenerFunc(
			func(e event.Connect) {
				eCount++
			}))

	suite.got.connectListeners.Visit(func(l event.ConnectListener) {
		l.OnConnect(event.Connect{})
	})
	suite.Equal(1, eCount)
}

func (suite *ProxySuite) TestOnQuicConnect() {

	// max tries not exceeded
	suite.mockQuicClient.On("Name").Return("quic")
	e := event.Connect{}
	suite.got.OnQuicConnect(e)
	suite.mockQuicClient.AssertNotCalled(suite.T(), "Stop")

	// max tries exceeded
	suite.mockQuicClient.On("Stop").Return()
	suite.mockWebsocket.On("Start").Return()
	e = event.Connect{
		TriesSinceLastConnect: 3,
		Err:                   errors.New("some error"),
	}
	suite.got.OnQuicConnect(e)

	suite.mockQuicClient.AssertCalled(suite.T(), "Stop")
	suite.mockWebsocket.AssertCalled(suite.T(), "Start")
}

func (suite *ProxySuite) TestName() {
	suite.mockQuicClient.On("Name").Return("quic")
	suite.Equal("quic", suite.got.Name())
}

func (suite *ProxySuite) TestOnWebsocketConnect() {

	// max tries not exceeded

	suite.mockWebsocket.On("Name").Return("websocket")
	e := event.Connect{}
	suite.got.OnWebsocketConnect(e)
	suite.mockQuicClient.AssertNotCalled(suite.T(), "Stop")

	// max tries exceeded

	suite.mockWebsocket.On("Stop").Return()
	suite.mockQuicClient.On("Start").Return()
	e = event.Connect{
		TriesSinceLastConnect: 3,
		Err:                   errors.New("some error"),
	}
	suite.got.OnWebsocketConnect(e)

	suite.mockWebsocket.AssertCalled(suite.T(), "Stop")
	suite.mockQuicClient.AssertCalled(suite.T(), "Start")

}

func (suite *ProxySuite) TestOnWebsocketConnectQuicDisabled() {
	// max tries exceeded
	suite.mockQuicClient.ExpectedCalls = nil
	suite.mockWebsocket.On("Stop").Return()
	suite.mockQuicClient.On("Start").Return()
	suite.mockQuicClient.On("IsEnabled").Return(false)

	suite.mockQuicClient.IsEnabled()

	e := event.Connect{
		TriesSinceLastConnect: 3,
		Err:                   errors.New("some error"),
	}

	suite.got.OnWebsocketConnect(e)

	suite.mockWebsocket.AssertNotCalled(suite.T(), "Stop")
	suite.mockQuicClient.AssertNotCalled(suite.T(), "Start")
}

func (suite *ProxySuite) TestProxyCalls() {

	suite.mockQuicClient.On("Start")
	suite.mockQuicClient.On("Stop")
	suite.mockQuicClient.On("AddConnectListener", mock.Anything).Return(TestCancelFunc)
	suite.mockQuicClient.On("AddMessageListener", mock.Anything).Return(TestCancelFunc)
	suite.mockQuicClient.On("HandleWrp", mock.Anything).Return(nil)

	suite.got.HandleWrp(wrp.Message{})

	suite.got.Start()
	suite.got.Stop()
	suite.got.AddConnectListener(
		event.ConnectListenerFunc(
			func(e event.Connect) {},
		))
	suite.got.AddMessageListener(
		event.MsgListenerFunc(
			func(msg wrp.Message) {},
		))

	suite.mockQuicClient.AssertCalled(suite.T(), "Start")
	suite.mockQuicClient.AssertCalled(suite.T(), "Stop")
	suite.mockQuicClient.AssertCalled(suite.T(), "HandleWrp", mock.Anything)
	suite.Equal(1, suite.got.msgListeners.Len())
	suite.Equal(1, suite.got.connectListeners.Len())
}
