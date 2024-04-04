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
	wshandler "github.com/xmidt-org/xmidt-agent/internal/wrphandlers/websocket"
	"go.uber.org/fx"
)

var (
	ErrWRPHandlerConfig = errors.New("wrphandler configuration error")
)

func provideWRPHandlers() fx.Option {
	return fx.Options(
		fx.Provide(
			provideWSHandler,
			providePubSubHandler,
			provideMissingHandler,
			provideAuthHandler,
		),
		fx.Invoke(provideWSEventorToHandlerAdapter),
	)
}

type wsAdapterIn struct {
	fx.In

	// Configuration
	WSConfg Websocket
	WS      *websocket.Websocket

	// wrphandlers
	AuthHandler             *auth.Handler
	WRPHandlerAdapterCancel event.CancelFunc
}

func provideWSEventorToHandlerAdapter(in wsAdapterIn) {
	if in.WSConfg.Disable {
		return
	}

	in.WS.AddMessageListener(
		event.MsgListenerFunc(func(m wrp.Message) {
			_ = in.AuthHandler.HandleWrp(m)
		}),
		&in.WRPHandlerAdapterCancel,
	)
}

type wsHandlerIn struct {
	fx.In

	// Configuration
	WSConfg Websocket

	WS *websocket.Websocket
}

func provideWSHandler(in wsHandlerIn) (h *wshandler.Handler, err error) {
	defer func() {
		if err != nil {
			err = errors.Join(ErrWRPHandlerConfig, err)
		}
	}()

	if in.WSConfg.Disable {
		return nil, nil
	}

	return wshandler.New(in.WS)
}

type missingIn struct {
	fx.In

	// Configuration
	// Note, DeviceID and PartnerID is pulled from the Identity configuration
	DeviceID wrp.DeviceID
	WSConfg  Websocket

	// wrphandlers
	WSHandler *wshandler.Handler
	Pubsub    *pubsub.PubSub
}

func provideMissingHandler(in missingIn) (h *missing.Handler, err error) {
	defer func() {
		if err != nil {
			err = errors.Join(ErrWRPHandlerConfig, err)
		}
	}()

	if in.WSConfg.Disable {
		return nil, nil
	}

	return missing.New(in.Pubsub, in.WSHandler, string(in.DeviceID))
}

type authIn struct {
	fx.In

	// Configuration
	// Note, DeviceID and PartnerID is pulled from the Identity configuration
	DeviceID  wrp.DeviceID
	PartnerID string
	WSConfg   Websocket

	// wrphandlers
	WSHandler      *wshandler.Handler
	MissingHandler *missing.Handler
}

func provideAuthHandler(in authIn) (h *auth.Handler, err error) {
	defer func() {
		if err != nil {
			err = errors.Join(ErrWRPHandlerConfig, err)
		}
	}()

	if in.WSConfg.Disable {
		return nil, nil
	}

	return auth.New(in.MissingHandler, in.WSHandler, string(in.DeviceID), in.PartnerID)
}

type pubsubIn struct {
	fx.In

	// Configuration
	// Note, DeviceID and PartnerID is pulled from the Identity configuration
	DeviceID  wrp.DeviceID
	Pubsub    Pubsub
	MockTr181 MockTr181
	WSConfg   Websocket

	// wrphandlers
	WSHandler *wshandler.Handler
}

type pubsubOut struct {
	fx.Out

	PubSub           *pubsub.PubSub
	PubSubCancelList []pubsub.CancelFunc
}

func providePubSubHandler(in pubsubIn) (out pubsubOut, err error) {
	defer func() {
		if err != nil {
			err = errors.Join(ErrWRPHandlerConfig, err)
		}
	}()

	if in.WSConfg.Disable {
		return pubsubOut{}, nil
	}

	var (
		egress     pubsub.CancelFunc
		cancelList = []pubsub.CancelFunc{egress}
	)

	opts := []pubsub.Option{
		pubsub.WithPublishTimeout(in.Pubsub.PublishTimeout),
		pubsub.WithEgressHandler(in.WSHandler, &egress),
	}

	var ps *pubsub.PubSub
	if in.MockTr181.Enabled {
		var mocktr pubsub.CancelFunc

		mockDefaults := []mocktr181.Option{
			mocktr181.FilePath("mock_tr181_test.json"),
			mocktr181.Enabled(in.MockTr181.Enabled),
		}
		mocktr181Handler, err := mocktr181.New(ps, string(in.DeviceID), mockDefaults...)
		if err != nil {
			return pubsubOut{}, nil
		}

		opts = append(opts,
			pubsub.WithServiceHandler("mocktr181", mocktr181Handler, &mocktr),
		)
		cancelList = append(cancelList, mocktr)
	}

	ps, err = pubsub.New(
		in.DeviceID,
		opts...,
	)

	return pubsubOut{
		PubSub:           ps,
		PubSubCancelList: cancelList,
	}, err
}
