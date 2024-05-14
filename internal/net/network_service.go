// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package net

import (
	"net"
	"sort"
	"strings"
)

type NetworkServicer interface {
	GetInterfaces() ([]net.Interface, error)
	GetInterfaceNames() ([]string, error)
}

type NetworkService struct {
	N                 NetworkWrapper
	AllowedInterfaces []string // should be supplied in priority order
}

func New(n NetworkWrapper, allowedInterfaces []string) NetworkServicer {
	return &NetworkService{
		N:                 n,
		AllowedInterfaces: allowedInterfaces,
	}
}

/** returns available interfaces in priority use order */
func (ns *NetworkService) GetInterfaces() ([]net.Interface, error) {
	ifaces, err := ns.N.Interfaces()
	if err != nil {
		return []net.Interface{}, err
	}

	var running []net.Interface
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagRunning != 0) && (ns.isAllowed(iface.Name)) {
			running = append(running, iface)
		}
	}

	sort.Slice(running, func(i, j int) bool {
		return ns.getPriority(running[i].Name) < ns.getPriority(running[j].Name)
	})

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

func (ns *NetworkService) isAllowed(name string) bool {
	for _, n := range ns.AllowedInterfaces {
		if strings.EqualFold(name, n) {
			return true
		}
	}

	return false
}

func (ns *NetworkService) getPriority(name string) int {
	for i, n := range ns.AllowedInterfaces {
		if strings.EqualFold(name, n) {
			return i
		}
	}

	return 100 // this should never happen
}
