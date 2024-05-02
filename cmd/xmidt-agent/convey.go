// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"time"

	"github.com/xmidt-org/xmidt-agent/internal/convey"
	"github.com/xmidt-org/xmidt-agent/internal/net"
	"go.uber.org/fx"
)

type conveyIn struct {
	fx.In
	NetworkService *net.NetworkService
	ID             Identity
	Ops            OperationalState
	Convey         Convey
}

func provideConveyHeaderProvider(in conveyIn) (*convey.ConveyHeaderProvider, error) {
	opts := []convey.Option{
		convey.NetworkServiceOpt(in.NetworkService),
		convey.FieldsOpt(in.Convey.Fields),
		convey.SerialNumberOpt(in.ID.SerialNumber),
		convey.HardwareModelOpt(in.ID.HardwareModel),
		convey.ManufacturerOpt(in.ID.HardwareManufacturer),
		convey.FirmwareOpt(in.ID.FirmwareVersion),
		convey.LastRebootReasonOpt(in.Ops.LastRebootReason),
		convey.XmidtProtocolOpt(xmidtProtocol),
		convey.BootTimeOpt(in.Ops.BootTime.String()),
		convey.BootRetryWaitOpt(time.Second), // should this be configured?
	}
	return convey.New(opts...)
}
