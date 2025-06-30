// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"time"

	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/credentials"
	"github.com/xmidt-org/xmidt-agent/internal/event"
	"github.com/xmidt-org/xmidt-agent/internal/jwtxt"
	"github.com/xmidt-org/xmidt-agent/internal/metadata"
	"github.com/xmidt-org/xmidt-agent/internal/websocket"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	ErrWebsocketConfig = errors.New("websocket configuration error")
)

type WsIn struct {
	fx.In
	Identity  Identity
	Logger    *zap.Logger
	CLI       *CLI
	JWTXT     *jwtxt.Instructions
	Cred      *credentials.Credentials
	Metadata  *metadata.MetadataProvider
	Cloud     Cloud
	Quic      Quic
	Websocket Websocket
}

type wsOut struct {
	fx.Out
	WS *websocket.Websocket

	// cancels
	Cancels []func() `group:"cancels,flatten"`
}

func provideWS(in WsIn) (wsOut, error) { //nolint

	var fetchURLFunc func(context.Context) (string, error)
	// JWTXT is not required
	// fetchURL() will use in.Websocket.BackUpURL if in.JWTXT is nil
	if in.JWTXT != nil {
		fetchURLFunc = in.JWTXT.Endpoint
	}

	var opts []websocket.Option
	// Allow operations where no credentials are desired (in.Cred will be nil).
	if in.Cred != nil {
		opts = append(opts, websocket.CredentialsDecorator(in.Cred.Decorate))
	}

	// Configuration options
	opts = append(opts,
		websocket.DeviceID(in.Identity.DeviceID),
		websocket.FetchURLTimeout(in.Websocket.FetchURLTimeout),
		websocket.FetchURL(
			fetchURL(in.Websocket.URLPath, in.Websocket.BackUpURL,
				fetchURLFunc)),
		websocket.InactivityTimeout(in.Websocket.InactivityTimeout),
		websocket.PingWriteTimeout(in.Websocket.PingWriteTimeout),
		websocket.SendTimeout(in.Websocket.SendTimeout),
		websocket.KeepAliveInterval(in.Websocket.KeepAliveInterval),
		websocket.HTTPClientWithForceSets(in.Websocket.HTTPClient),
		websocket.MaxMessageBytes(in.Websocket.MaxMessageBytes),
		websocket.ConveyDecorator(in.Metadata.Decorate),
		websocket.ConveyMsgDecorator(in.Metadata.DecorateMsg),
		websocket.AdditionalHeaders(in.Websocket.AdditionalHeaders),
		websocket.NowFunc(time.Now),
		websocket.WithIPv6(!in.Websocket.DisableV6),
		websocket.WithIPv4(!in.Websocket.DisableV4),
		websocket.Once(in.Websocket.Once),
		websocket.RetryPolicy(in.Websocket.RetryPolicy),
	)

	// Listener options
	var (
		msg, con, discon, heartbeat event.CancelFunc
		cancels                     []func()
	)
	if in.CLI.Dev {
		logger := in.Logger.Named("websocket")
		opts = append(opts,
			websocket.AddMessageListener(
				event.MsgListenerFunc(
					func(m wrp.Message) {
						logger.Info("message listener", zap.Any("msg", m))
					}), &msg),
			websocket.AddConnectListener(
				event.ConnectListenerFunc(
					func(e event.Connect) {
						logger.Info("connect listener", zap.Any("event", e))
					}), &con),
			websocket.AddDisconnectListener(
				event.DisconnectListenerFunc(
					func(e event.Disconnect) {
						logger.Info("disconnect listener", zap.Any("event", e))
					}), &discon),
			websocket.AddHeartbeatListener(
				event.HeartbeatListenerFunc(func(e event.Heartbeat) {
					logger.Info("heartbeat listener", zap.Any("event", e))
				}), &heartbeat),
		)
	}

	ws, err := websocket.New(opts...)
	if err != nil {
		err = errors.Join(ErrWebsocketConfig, err)
	}

	if in.CLI.Dev {
		cancels = append(cancels, msg, con, discon, heartbeat)
	}

	return wsOut{
		WS:      ws,
		Cancels: cancels,
	}, err
}
