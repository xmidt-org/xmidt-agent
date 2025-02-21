// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"time"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/credentials"
	"github.com/xmidt-org/xmidt-agent/internal/jwtxt"
	"github.com/xmidt-org/xmidt-agent/internal/metadata"
	"github.com/xmidt-org/xmidt-agent/internal/quic"
	"github.com/xmidt-org/xmidt-agent/internal/websocket/event"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var (
	ErrQuicConfig = errors.New("quic configuration error")
)

type QuicIn struct {
	fx.In
	Identity   Identity
	Logger     *zap.Logger
	CLI        *CLI
	JWTXT      *jwtxt.Instructions
	Cred       *credentials.Credentials
	Metadata   *metadata.MetadataProvider
	Quic Quic
}

type quicOut struct {
	fx.Out
	//QuicHandler wrpkit.Handler  // duplicate with Websocket
	QuicClient  *quic.QuicClient
	Egress      quic.Egress

	// cancels
	Cancels []func() `group:"cancels,flatten"`
}

func provideQuic(in QuicIn) (quicOut, error) {
	if in.Quic.Disable {
		return quicOut{}, nil
	}

	var fetchURLFunc func(context.Context) (string, error)
	// JWTXT is not required
	// fetchURL() will use in.quic.BackUpURL if in.JWTXT is nil
	if in.JWTXT != nil {
		fetchURLFunc = in.JWTXT.Endpoint
	}

	var opts []quic.Option
	// Allow operations where no credentials are desired (in.Cred will be nil).
	if in.Cred != nil {
		opts = append(opts, quic.CredentialsDecorator(in.Cred.Decorate))
	}

	// Configuration options
	opts = append(opts,
		quic.DeviceID(in.Identity.DeviceID),
		quic.FetchURLTimeout(in.Quic.FetchURLTimeout),
		quic.FetchURL(
			fetchURL(in.Quic.URLPath, in.Quic.BackUpURL,
				fetchURLFunc)),
		quic.InactivityTimeout(in.Quic.InactivityTimeout),
		quic.PingWriteTimeout(in.Quic.PingWriteTimeout),
		quic.SendTimeout(in.Quic.SendTimeout),
		quic.KeepAliveInterval(in.Quic.KeepAliveInterval),
		quic.HTTPClientWithForceSets(in.Quic.HTTPClient),
		quic.MaxMessageBytes(in.Quic.MaxMessageBytes),
		quic.ConveyDecorator(in.Metadata.Decorate),
		quic.AdditionalHeaders(in.Quic.AdditionalHeaders),
		quic.NowFunc(time.Now),
		quic.WithIPv6(!in.Quic.DisableV6),
		quic.WithIPv4(!in.Quic.DisableV4),
		quic.Once(in.Quic.Once),
		quic.RetryPolicy(in.Quic.RetryPolicy),
	)

	// Listener options
	var (
		msg, con, discon, heartbeat event.CancelFunc
		cancels                     []func()
	)
	if in.CLI.Dev {
		logger := in.Logger.Named("quic")
		opts = append(opts,
			quic.AddMessageListener(
				event.MsgListenerFunc(
					func(m wrp.Message) {
						logger.Info("message listener", zap.Any("msg", m))
					}), &msg),
			quic.AddConnectListener(
				event.ConnectListenerFunc(
					func(e event.Connect) {
						logger.Info("connect listener", zap.Any("event", e))
					}), &con),
			quic.AddDisconnectListener(
				event.DisconnectListenerFunc(
					func(e event.Disconnect) {
						logger.Info("disconnect listener", zap.Any("event", e))
					}), &discon),
			quic.AddHeartbeatListener(
				event.HeartbeatListenerFunc(func(e event.Heartbeat) {
					logger.Info("heartbeat listener", zap.Any("event", e))
				}), &heartbeat),
		)
	}

	quicClient, err := quic.New(opts...)
	if err != nil {
		err = errors.Join(ErrQuicConfig, err)
	}

	if in.CLI.Dev {
		cancels = append(cancels, msg, con, discon, heartbeat) 
	}

	return quicOut{
		QuicClient:      quicClient,
		Egress:  quicClient,
		Cancels: cancels, 
	}, err
}

// TODO - duplicated in ws.go
// func fetchURL(path, backUpURL string, f func(context.Context) (string, error)) func(context.Context) (string, error) {
// 	return func(ctx context.Context) (string, error) {
// 		if f == nil {
// 			return url.JoinPath(backUpURL, path)
// 		}

// 		baseURL, err := f(ctx)
// 		if err != nil {
// 			if backUpURL != "" {
// 				return url.JoinPath(backUpURL, path)
// 			}

// 			return "", err
// 		}

// 		return url.JoinPath(baseURL, path)
// 	}
// }
