// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"errors"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/pubsub"
	"github.com/xmidt-org/xmidt-agent/internal/websocket"
	"github.com/xmidt-org/xmidt-agent/internal/websocket/event"
	"github.com/xmidt-org/xmidt-agent/internal/wrphandlers/auth"
	"github.com/xmidt-org/xmidt-agent/internal/wrphandlers/missing"
	"github.com/xmidt-org/xmidt-agent/internal/wrphandlers/mocktr181"
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
		),
		fx.Invoke(provideWSEventorToHandlerAdapter),
	)
}

type wsAdapterIn struct {
	fx.In

	WS *websocket.Websocket

	// wrphandlers
	AuthHandler             *auth.Handler
	WRPHandlerAdapterCancel event.CancelFunc
}

func provideWSEventorToHandlerAdapter(in wsAdapterIn) {
	in.WS.AddMessageListener(
		event.MsgListenerFunc(func(m wrp.Message) {
			_ = in.AuthHandler.HandleWrp(m)
		}),
		&in.WRPHandlerAdapterCancel,
	)
}

type missingIn struct {
	fx.In

	// Configuration
	// Note, DeviceID and PartnerID is pulled from the Identity configuration
	Identity Identity

	// wrphandlers
	Egress websocket.Egress
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

	Egress         websocket.Egress
	MissingHandler *missing.Handler
}

func provideAuthHandler(in authIn) (*auth.Handler, error) {
	h, err := auth.New(in.MissingHandler, in.Egress, string(in.Identity.DeviceID), in.Identity.PartnerID)
	if err != nil {
		err = errors.Join(ErrWRPHandlerConfig, err)
	}

	return h, err
}

type pubsubIn struct {
	fx.In

	// Configuration
	// Note, DeviceID and PartnerID is pulled from the Identity configuration
	Identity  Identity
	Pubsub    Pubsub
	MockTr181 MockTr181

	// wrphandlers
	Egress websocket.Egress
}

type pubsubOut struct {
	fx.Out

	PubSub           *pubsub.PubSub
	PubSubCancelList []pubsub.CancelFunc
}

func providePubSubHandler(in pubsubIn) (pubsubOut, error) {
	var (
		egress     pubsub.CancelFunc
		cancelList = []pubsub.CancelFunc{egress}
	)

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

	if in.MockTr181.Enabled {
		mockDefaults := []mocktr181.Option{
			mocktr181.FilePath(in.MockTr181.FilePath),
			mocktr181.Enabled(in.MockTr181.Enabled),
		}
		mocktr181Handler, err := mocktr181.New(ps, string(in.Identity.DeviceID), mockDefaults...)
		if err != nil {
			return pubsubOut{}, errors.Join(ErrWRPHandlerConfig, err)
		}

		mocktr, err := ps.SubscribeService(in.MockTr181.ServiceName, mocktr181Handler)
		if err != nil {
			return pubsubOut{}, errors.Join(ErrWRPHandlerConfig, err)
		}

		cancelList = append(cancelList, mocktr)
	}

	return pubsubOut{
		PubSub:           ps,
		PubSubCancelList: cancelList,
	}, err
}
