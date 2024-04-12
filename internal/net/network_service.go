package net

import (
	"net"
)

type NetworkService struct {
	n NetworkInterface
}

func New(n NetworkInterface) *NetworkService {
	return &NetworkService{
		n: n,
	}
}

func (ns *NetworkService) GetInterfaceNames() ([]string, error) {
	ifaces, err := ns.n.Interfaces()
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
