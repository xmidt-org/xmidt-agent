// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package net

import (
	"github.com/nixigaj/go-default-route"
	"net"
)

type NetworkWrap struct{}

type NetworkWrapper interface {
	Interfaces() ([]net.Interface, error)
	DefaultInterface() (*net.Interface, error)
}

func NewNetworkWrapper() NetworkWrapper {
	return new(NetworkWrap)
}

func (n *NetworkWrap) Interfaces() ([]net.Interface, error) {
	return net.Interfaces()
}

func (n *NetworkWrap) DefaultInterface() (*net.Interface, error) {
	return defaultroute.DefaultRouteInterface()
}
