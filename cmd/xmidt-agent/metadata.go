// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"time"

	"github.com/xmidt-org/xmidt-agent/internal/metadata"

	"github.com/xmidt-org/xmidt-agent/internal/net"
	"go.uber.org/fx"
)

type metadataIn struct {
	fx.In
	NetworkService net.NetworkServicer
	ID             Identity
	Ops            OperationalState
	Metadata       Metadata
}

func provideMetadataProvider(in metadataIn) (*metadata.MetadataProvider, error) {
	opts := []metadata.Option{
		metadata.NetworkServiceOpt(in.NetworkService),
		metadata.FieldsOpt(in.Metadata.Fields),
		metadata.SerialNumberOpt(in.ID.SerialNumber),
		metadata.HardwareModelOpt(in.ID.HardwareModel),
		metadata.ManufacturerOpt(in.ID.HardwareManufacturer),
		metadata.FirmwareOpt(in.ID.FirmwareVersion),
		metadata.LastRebootReasonOpt(in.Ops.LastRebootReason),
		metadata.XmidtProtocolOpt(xmidtProtocol),
		metadata.BootTimeOpt(in.Ops.BootTime.String()),
		metadata.BootRetryWaitOpt(time.Second), // should this be configured?
		metadata.InterfaceUsedOpt(in.Ops.WebpaInterfaceUsed),
	}
	return metadata.New(opts...)
}
