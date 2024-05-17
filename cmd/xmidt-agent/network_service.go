// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/xmidt-org/xmidt-agent/internal/net"
	"go.uber.org/fx"
)

type networkServiceIn struct {
	fx.In
	NetworkService NetworkService
}

func provideNetworkService(in networkServiceIn) net.NetworkServicer {
	return net.New(net.NewNetworkWrapper(), in.NetworkService.AllowedInterfaces)
}
