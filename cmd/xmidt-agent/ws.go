// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/credentials"
	"github.com/xmidt-org/xmidt-agent/internal/jwtxt"
	"github.com/xmidt-org/xmidt-agent/internal/websocket"
	"github.com/xmidt-org/xmidt-agent/internal/websocket/event"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	ErrWebsocketConfig = errors.New("websocket configuration error")
)

type wsIn struct {
	fx.In
	// Note, DeviceID is pulled from the Identity configuration
	DeviceID  wrp.DeviceID
	Logger    *zap.Logger
	CLI       *CLI
	JWTXT     *jwtxt.Instructions
	Cred      *credentials.Credentials
	Websocket Websocket
}

type wsOut struct {
	fx.Out
	WS         *websocket.Websocket
	CancelList []event.CancelFunc
}

func provideWS(in wsIn) (wsOut, error) {
	if in.Websocket.Disable {
		return wsOut{}, nil
	}

	opts := []websocket.Option{
		websocket.DeviceID(in.DeviceID),
		websocket.FetchURLTimeout(in.Websocket.FetchURLTimeout),
		websocket.FetchURL(fetchURL(in.Websocket.URLPath, in.JWTXT.Endpoint)),
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

	var (
		cancelList       []event.CancelFunc
		msg, con, discon event.CancelFunc
	)
	if in.CLI.Dev {
		opts = append(opts,
			websocket.AddMessageListener(
				event.MsgListenerFunc(
					func(m wrp.Message) {
						in.Logger.Info("message listener", zap.Any("msg", m))
					}), &msg),
			websocket.AddConnectListener(
				event.ConnectListenerFunc(
					func(e event.Connect) {
						in.Logger.Info("connect listener", zap.Any("event", e))
					}), &con),
			websocket.AddDisconnectListener(
				event.DisconnectListenerFunc(
					func(e event.Disconnect) {
						in.Logger.Info("disconnect listener", zap.Any("event", e))
					}), &discon),
		)
		cancelList = append(cancelList, msg, con, discon)
	}

	ws, err := websocket.New(opts...)
	if err != nil {
		err = fmt.Errorf("%w: %s", ErrWebsocketConfig, err)
	}

	return wsOut{
		WS:         ws,
		CancelList: cancelList,
	}, err
}

func fetchURL(path string, f func(context.Context) (string, error)) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		baseURL, err := f(ctx)
		if err != nil {
			return "", err
		}

		return url.JoinPath(baseURL, path)
	}
}
