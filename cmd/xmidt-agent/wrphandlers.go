// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"errors"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/loglevel"
	"github.com/xmidt-org/xmidt-agent/internal/pubsub"
	"github.com/xmidt-org/xmidt-agent/internal/websocket"
	"github.com/xmidt-org/xmidt-agent/internal/websocket/event"
	"github.com/xmidt-org/xmidt-agent/internal/wrphandlers/auth"
	"github.com/xmidt-org/xmidt-agent/internal/wrphandlers/missing"
	"github.com/xmidt-org/xmidt-agent/internal/wrphandlers/mocktr181"
	"github.com/xmidt-org/xmidt-agent/internal/wrphandlers/qos"
	"github.com/xmidt-org/xmidt-agent/internal/wrphandlers/xmidt_agent_crud"
	"go.uber.org/fx"
)

var (
	ErrWRPHandlerConfig = errors.New("wrphandler configuration error")
)

func provideWRPHandlers() fx.Option {
	return fx.Options(
		fx.Provide(
			providePubSubHandler,
			provideMissingHandler,
			provideAuthHandler,
			provideCrudHandler,
			provideQOSHandler,
			provideWSEventorToHandlerAdapter,
			provideMockTr181Handler,
		),
	)
}

type wsAdapterIn struct {
	fx.In

	WS *websocket.Websocket

	// wrphandlers
	AuthHandler *auth.Handler
}

type wsAdapterOut struct {
	fx.Out

	Cancels []func() `group:"cancels,flatten"`
}

func provideWSEventorToHandlerAdapter(in wsAdapterIn) wsAdapterOut {
	return wsAdapterOut{
		Cancels: []func(){
			in.WS.AddMessageListener(
				event.MsgListenerFunc(func(m wrp.Message) {
					_ = in.AuthHandler.HandleWrp(m)
				}),
			),
		}}
}

type qosIn struct {
	fx.In

	QOS QOS
	WS  *websocket.Websocket
}

func provideQOSHandler(in qosIn) (*qos.Handler, error) {
	return qos.New(
		in.WS,
		qos.MaxQueueBytes(in.QOS.MaxQueueBytes),
		qos.MaxMessageBytes(in.QOS.MaxMessageBytes),
	)
}

type missingIn struct {
	fx.In

	// Configuration
	// Note, DeviceID and PartnerID is pulled from the Identity configuration
	Identity Identity

	// wrphandlers
	Egress *qos.Handler
	Pubsub *pubsub.PubSub
}

func provideMissingHandler(in missingIn) (*missing.Handler, error) {
	h, err := missing.New(in.Pubsub, in.Egress, string(in.Identity.DeviceID))
	if err != nil {
		err = errors.Join(ErrWRPHandlerConfig, err)
	}

	return h, err
}

type authIn struct {
	fx.In

	// Configuration
	// Note, DeviceID and PartnerID is pulled from the Identity configuration
	Identity Identity

	// wrphandlers

	Egress         *qos.Handler
	MissingHandler *missing.Handler
}

func provideAuthHandler(in authIn) (*auth.Handler, error) {
	h, err := auth.New(in.MissingHandler, in.Egress, string(in.Identity.DeviceID), in.Identity.PartnerID)
	if err != nil {
		err = errors.Join(ErrWRPHandlerConfig, err)
	}

	return h, err
}

type crudIn struct {
	fx.In

	XmidtAgentCrud  XmidtAgentCrud
	Identity        Identity
	Egress          websocket.Egress
	LogLevelService *loglevel.LogLevelService
	PubSub          *pubsub.PubSub
}

func provideCrudHandler(in crudIn) (*xmidt_agent_crud.Handler, error) {
	h, err := xmidt_agent_crud.New(in.Egress, string(in.Identity.DeviceID), in.LogLevelService)
	if err != nil {
		err = errors.Join(ErrWRPHandlerConfig, err)
		return nil, err
	}

	// what do we do with the return value?  Does this have to be passed to pubSub somehow? Seems
	// fishy if it does
	_, err = in.PubSub.SubscribeService(in.XmidtAgentCrud.ServiceName, h)
	if err != nil {
		return nil, errors.Join(ErrWRPHandlerConfig, err)
	}

	return h, err
}

type pubsubIn struct {
	fx.In

	// Configuration
	// Note, DeviceID and PartnerID is pulled from the Identity configuration
	Identity Identity
	Pubsub   Pubsub

	// wrphandlers
	Egress *qos.Handler
}

type pubsubOut struct {
	fx.Out

	PubSub *pubsub.PubSub
	Cancel func() `group:"cancels"`
}

func providePubSubHandler(in pubsubIn) (pubsubOut, error) {
	var egress pubsub.CancelFunc

	opts := []pubsub.Option{
		pubsub.WithPublishTimeout(in.Pubsub.PublishTimeout),
		pubsub.WithEgressHandler(in.Egress, &egress),
	}
	ps, err := pubsub.New(
		in.Identity.DeviceID,
		opts...,
	)
	if err != nil {
		return pubsubOut{}, errors.Join(ErrWRPHandlerConfig, err)
	}

	return pubsubOut{
		PubSub: ps,
		Cancel: egress,
	}, err
}

type mockTr181In struct {
	fx.In

	// Configuration
	// Note, DeviceID and PartnerID is pulled from the Identity configuration
	Identity  Identity
	MockTr181 MockTr181

	PubSub *pubsub.PubSub
}

type mockTr181Out struct {
	fx.Out
	Cancel func() `group:"cancels"`
}

func provideMockTr181Handler(in mockTr181In) (mockTr181Out, error) {
	if !in.MockTr181.Enabled {
		return mockTr181Out{}, nil
	}

	mockDefaults := []mocktr181.Option{
		mocktr181.FilePath(in.MockTr181.FilePath),
		mocktr181.Enabled(in.MockTr181.Enabled),
	}
	mocktr181Handler, err := mocktr181.New(in.PubSub, string(in.Identity.DeviceID), mockDefaults...)
	if err != nil {
		return mockTr181Out{}, errors.Join(ErrWRPHandlerConfig, err)
	}

	mocktr, err := in.PubSub.SubscribeService(in.MockTr181.ServiceName, mocktr181Handler)
	if err != nil {
		return mockTr181Out{}, errors.Join(ErrWRPHandlerConfig, err)
	}

	return mockTr181Out{
		Cancel: mocktr,
	}, nil
}
