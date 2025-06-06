// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"time"

	"github.com/xmidt-org/wrp-go/v5"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/xmidt-org/xmidt-agent/internal/credentials"
	"github.com/xmidt-org/xmidt-agent/internal/event"
	"github.com/xmidt-org/xmidt-agent/internal/jwtxt"
	"github.com/xmidt-org/xmidt-agent/internal/metadata"
	"github.com/xmidt-org/xmidt-agent/internal/quic"
)

var (
	ErrQuicConfig = errors.New("quic configuration error")
)

type QuicIn struct {
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

type quicOut struct {
	fx.Out
	QuicClient *quic.QuicClient

	// cancels
	Cancels []func() `group:"cancels,flatten"`
}

func provideQuic(in QuicIn) (quicOut, error) { //nolint
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
		quic.Enabled(!in.Quic.Disable),
		quic.DeviceID(in.Identity.DeviceID),
		quic.FetchURLTimeout(in.Quic.FetchURLTimeout),
		quic.FetchURL(
			fetchURL(in.Quic.URLPath, in.Quic.BackUpURL,
				fetchURLFunc)),
		quic.SendTimeout(in.Quic.SendTimeout),
		quic.HTTP3Client(&in.Quic.QuicClient),
		quic.ConveyDecorator(in.Metadata.Decorate),
		quic.AdditionalHeaders(in.Quic.AdditionalHeaders),
		quic.NowFunc(time.Now),
		quic.Once(in.Quic.Once),
		quic.RetryPolicy(in.Quic.RetryPolicy),
	)

	// Listener options
	var (
		msg, con, discon event.CancelFunc
		cancels          []func()
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
		)
	}

	quicClient, err := quic.New(opts...)
	if err != nil {
		err = errors.Join(ErrQuicConfig, err)
	}

	if in.CLI.Dev {
		cancels = append(cancels, msg, con, discon)
	}

	return quicOut{
		QuicClient: quicClient,
		Cancels:    cancels,
	}, err
}
