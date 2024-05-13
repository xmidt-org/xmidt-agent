// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package net

import (
	"net"
)

type NetworkServicer interface {
	GetInterfaces() ([]net.Interface, error)
	GetInterfaceNames() ([]string, error)
}

type NetworkService struct {
	N NetworkWrapper
}

func New(n NetworkWrapper) NetworkServicer {
	return &NetworkService{
		N: n,
	}
}

func (ns *NetworkService) GetInterfaces() ([]net.Interface, error) {
	ifaces, err := ns.N.Interfaces()
	if err != nil {
		return []net.Interface{}, err
	}

	var running []net.Interface
	for _, iface := range ifaces {
		if iface.Flags&net.FlagRunning != 0 {
			running = append(running, iface)
		}
	}

	return running, nil
}

func (ns *NetworkService) GetInterfaceNames() ([]string, error) {
	ifaces, err := ns.GetInterfaces()
	if err != nil {
		return []string{}, err
	}

	names := []string{}
	for _, iface := range ifaces {
		names = append(names, iface.Name)
	}

	return names, nil
}
