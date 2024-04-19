// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/credentials"
	"github.com/xmidt-org/xmidt-agent/internal/jwtxt"
	"github.com/xmidt-org/xmidt-agent/internal/websocket"
	"github.com/xmidt-org/xmidt-agent/internal/websocket/event"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	ErrWebsocketConfig = errors.New("websocket configuration error")
)

type wsIn struct {
	fx.In
	// Note, DeviceID is pulled from the Identity configuration
	Identity  Identity
	Logger    *zap.Logger
	CLI       *CLI
	JWTXT     *jwtxt.Instructions
	Cred      *credentials.Credentials
	Websocket Websocket
}

type wsOut struct {
	fx.Out
	WSHandler               wrpkit.Handler
	WS                      *websocket.Websocket
	Egress                  websocket.Egress
	WRPHandlerAdapterCancel event.CancelFunc
	EventCancelList         []event.CancelFunc
}

func provideWS(in wsIn) (wsOut, error) {
	if in.Websocket.Disable {
		return wsOut{}, nil
	}

	// Configuration options
	opts := []websocket.Option{
		websocket.DeviceID(in.Identity.DeviceID),
		websocket.FetchURLTimeout(in.Websocket.FetchURLTimeout),
		websocket.FetchURL(
			fetchURL(in.Websocket.URLPath, in.Websocket.BackUpURL,
				in.JWTXT.Endpoint)),
		websocket.PingInterval(in.Websocket.PingInterval),
		websocket.PingTimeout(in.Websocket.PingTimeout),
		websocket.ConnectTimeout(in.Websocket.ConnectTimeout),
		websocket.KeepAliveInterval(in.Websocket.KeepAliveInterval),
		websocket.IdleConnTimeout(in.Websocket.IdleConnTimeout),
		websocket.TLSHandshakeTimeout(in.Websocket.TLSHandshakeTimeout),
		websocket.ExpectContinueTimeout(in.Websocket.ExpectContinueTimeout),
		websocket.MaxMessageBytes(in.Websocket.MaxMessageBytes),
		websocket.CredentialsDecorator(in.Cred.Decorate),
		websocket.AdditionalHeaders(in.Websocket.AdditionalHeaders),
		websocket.NowFunc(time.Now),
		websocket.WithIPv6(!in.Websocket.DisableV6),
		websocket.WithIPv4(!in.Websocket.DisableV4),
		websocket.Once(in.Websocket.Once),
		websocket.RetryPolicy(in.Websocket.RetryPolicy),
	}

	// Listener options
	var (
		msg, con, discon, heartbeat, wrphandlerAdapter event.CancelFunc
		cancelList                                     = []event.CancelFunc{wrphandlerAdapter}
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
		cancelList = append(cancelList, msg, con, discon, heartbeat)
	}

	ws, err := websocket.New(opts...)
	if err != nil {
		err = errors.Join(ErrWebsocketConfig, err)
	}

	return wsOut{
		WS:                      ws,
		EventCancelList:         cancelList,
		WRPHandlerAdapterCancel: wrphandlerAdapter,
		Egress:                  ws,
	}, err
}

func fetchURL(path, backUpURL string, f func(context.Context) (string, error)) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		baseURL, err := f(ctx)
		if err != nil {
			if backUpURL != "" {
				return url.JoinPath(backUpURL, path)
			}

			return "", err
		}

		return url.JoinPath(baseURL, path)
	}
}
