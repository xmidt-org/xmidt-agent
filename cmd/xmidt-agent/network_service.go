// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/xmidt-org/xmidt-agent/internal/net"
	"go.uber.org/fx"
)

type NetworkServiceIn struct {
	fx.In
}

type NetworkServiceOut struct {
	fx.Out
	NetworkService *net.NetworkService
}

var NetworkServiceModule = fx.Module("networkService",
	fx.Provide(
		func(in NetworkServiceIn) *net.NetworkService {
			return &net.NetworkService{
				N: net.NewNetworkWrapper(),
			}
		}),
)
