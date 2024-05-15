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
	"github.com/xmidt-org/xmidt-agent/internal/metadata"
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
	Metadata  *metadata.MetadataProvider
	Websocket Websocket
}

type wsOut struct {
	fx.Out
	WSHandler wrpkit.Handler
	WS        *websocket.Websocket
	Egress    websocket.Egress

	// cancels
	Cancels []func() `group:"cancels,flatten"`
}

func provideWS(in wsIn) (wsOut, error) {
	if in.Websocket.Disable {
		return wsOut{}, nil
	}

	var fetchURLFunc func(context.Context) (string, error)
	// JWTXT is not required
	// fetchURL() will use in.Websocket.BackUpURL if in.JWTXT is nil
	if in.JWTXT != nil {
		fetchURLFunc = in.JWTXT.Endpoint
	}

	client, err := in.Websocket.HTTPClient.NewClient()
	if err != nil {
		return wsOut{}, err
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
		websocket.PingInterval(in.Websocket.PingInterval),
		websocket.PingTimeout(in.Websocket.PingTimeout),
		websocket.SendTimeout(in.Websocket.SendTimeout),
		websocket.KeepAliveInterval(in.Websocket.KeepAliveInterval),
		websocket.HTTPClient(client),
		websocket.MaxMessageBytes(in.Websocket.MaxMessageBytes),
		websocket.ConveyDecorator(in.Metadata.Decorate),
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
		Egress:  ws,
		Cancels: cancels,
	}, err
}

func fetchURL(path, backUpURL string, f func(context.Context) (string, error)) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		if f == nil {
			return url.JoinPath(backUpURL, path)
		}

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
