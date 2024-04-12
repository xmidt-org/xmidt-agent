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
