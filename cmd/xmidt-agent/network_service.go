// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"go.uber.org/fx"

	"github.com/xmidt-org/xmidt-agent/internal/net"
)

type networkServiceIn struct {
	fx.In
	NetworkService NetworkService
}

func provideNetworkService(in networkServiceIn) net.NetworkServicer {
	return net.New(net.NewNetworkWrapper(), in.NetworkService.AllowedInterfaces)
}
