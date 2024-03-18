// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"time"

	"github.com/xmidt-org/xmidt-agent/internal/credentials"
	"github.com/xmidt-org/xmidt-agent/internal/credentials/event"
	"github.com/xmidt-org/xmidt-agent/internal/fs"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	xmidtProtocol = "protocol"
)

type credsIn struct {
	fx.In
	Creds   XmidtCredentials
	ID      Identity
	Ops     OperationalState
	Durable fs.FS `name:"durable_fs" optional:"true"`
	LC      fx.Lifecycle
	Logger  *zap.Logger
}

func (in credsIn) Options() ([]credentials.Option, error) {
	logger := in.Logger.Named("credentials")

	// If the URL is empty, then there is no credentials service to use.
	if in.Creds.URL == "" {
		logger.Warn("no credentials service configured")
		return nil, nil
	}

	client, err := in.Creds.HTTPClient.NewClient()
	if err != nil {
		return nil, err
	}

	opts := []credentials.Option{
		credentials.URL(in.Creds.URL),
		credentials.HTTPClient(client),
		credentials.MacAddress(in.ID.DeviceID),
		credentials.SerialNumber(in.ID.SerialNumber),
		credentials.HardwareModel(in.ID.HardwareModel),
		credentials.HardwareManufacturer(in.ID.HardwareManufacturer),
		credentials.FirmwareVersion(in.ID.FirmwareVersion),
		credentials.PartnerID(func() string { return in.ID.PartnerID }),
		credentials.LastRebootReason(in.Ops.LastRebootReason),
		credentials.XmidtProtocol(xmidtProtocol),
		credentials.BootRetryWait(time.Second),
		credentials.RefetchPercent(in.Creds.RefetchPercent),
		credentials.AddFetchListener(event.FetchListenerFunc(
			func(e event.Fetch) {
				logger.Info("fetch",
					zap.String("origin", e.Origin),
					zap.Time("at", e.At),
					zap.Duration("duration", e.Duration),
					zap.String("uuid", e.UUID.String()),
					zap.Int("status_code", e.StatusCode),
					zap.Duration("retry_in", e.RetryIn),
					zap.Time("expiration", e.Expiration),
					zap.Error(e.Err),
				)
			})),
	}

	if in.Durable != nil {
		opts = append(opts,
			credentials.LocalStorage(in.Durable, in.Creds.FileName, in.Creds.FilePermissions),
		)
	}

	return opts, nil
}

func provideCredentials(in credsIn) (*credentials.Credentials, error) {
	opts, err := in.Options()
	if err != nil || opts == nil {
		return nil, err
	}
	creds, err := credentials.New(opts...)
	if err != nil {
		return nil, err
	}

	in.LC.Append(fx.Hook{
		OnStart: func(context.Context) error {
			creds.Start()
			return nil
		},
		OnStop: func(context.Context) error {
			creds.Stop()
			return nil
		},
	})

	return creds, nil
}
