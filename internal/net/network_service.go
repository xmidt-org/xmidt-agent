// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package net

import (
	"net"
)

type NetworkServicer interface {
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

func (ns *NetworkService) GetInterfaceNames() ([]string, error) {
	ifaces, err := ns.N.Interfaces()
	if err != nil {
		return []string{}, err
	}

	m := make(map[string]bool)
	for _, i := range ifaces {
		if i.Flags&net.FlagRunning != 0 {
			m[i.Name] = true
		}
	}

	names := []string{}
	for name := range m {
		names = append(names, name)
	}

	return names, nil
}
