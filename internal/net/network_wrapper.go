// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package net

import (
	"net"
)

type NetworkWrapper struct{}

type NetworkInterface interface {
	Interfaces() ([]net.Interface, error)
}

func NewNetworkWrapper() NetworkInterface {
	return new(NetworkService)
}

func (n *NetworkService) Interfaces() ([]net.Interface, error) {
	return net.Interfaces()
}
