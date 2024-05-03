// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/xmidt-org/xmidt-agent/internal/net"
)

func provideNetworkService() net.NetworkServicer {
	return &net.NetworkService{
		N: net.NewNetworkWrapper(),
	}
}
