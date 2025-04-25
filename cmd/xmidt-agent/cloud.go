// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/xmidt-org/xmidt-agent/internal/cloud"
	"github.com/xmidt-org/xmidt-agent/internal/event"
	"github.com/xmidt-org/xmidt-agent/internal/quic"
	"github.com/xmidt-org/xmidt-agent/internal/websocket"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

var (
	ErrCloudConfig = errors.New("cloud network configuration error")
)

type CloudHandlerIn struct {
	fx.In
	Logger     *zap.Logger
	CLI        *CLI
	Cloud      Cloud
	QuicClient *quic.QuicClient
	Websocket  *websocket.Websocket
}

type cloudHandlerOut struct {
	fx.Out
	CloudHandler cloud.Handler
	WrpHandler   wrpkit.Handler

	// cancels
	Cancels []func() `group:"cancels,flatten"`
}

func provideCloudHandler(in CloudHandlerIn) (cloudHandlerOut, error) {
	var opts []cloud.Option
	opts = append(opts,
		cloud.PreferQuic(in.Cloud.PreferQuic),
		cloud.QuicClient(in.QuicClient),
		cloud.Websocket(in.Websocket),
		cloud.MaxTries(in.Cloud.MaxTries),
	)

	var (
		msg, con, discon event.CancelFunc
		cancels          []func()
	)

	cloudProxy, err := cloud.New(opts...)
	if err != nil {
		err = errors.Join(ErrCloudConfig, err)
	}

	if in.CLI.Dev {
		cancels = append(cancels, msg, con, discon)
	}

	return cloudHandlerOut{
		CloudHandler: cloudProxy,
		WrpHandler:   cloudProxy.(wrpkit.Handler),
		Cancels:      cancels,
	}, err
}
