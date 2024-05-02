// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package convey

import (
	"fmt"
	"github.com/xmidt-org/xmidt-agent/internal/net"
)

type Option interface {
	apply(*ConveyHeaderProvider) error
}

type optionFunc func(*ConveyHeaderProvider) error

func (f optionFunc) apply(c *ConveyHeaderProvider) error {
	return f(c)
}

const (
	HeaderName = "X-Webpa-Convey"
)

const (
	Firmware                   = "fw-name"
	Hardware                   = "hw-model"
	Manufacturer               = "hw-manufacturer"
	SerialNumber               = "hw-serial-number"
	LastRebootReason           = "hw-last-reboot-reason"
	Protocol                   = "webpa-protocol"
	BootTime                   = "boot-time"
	BootTimeRetryDelay         = "boot-time-retry-wait"
	InterfaceUsed       string = "webpa-interface-used"
	InterfacesAvailable        = "interfaces-available"
)

type ConveyHeaderProvider struct {
	networkService     *net.NetworkService
	fields             []string
	firmware           string
	hardware           string
	manufacturer       string
	serialNumber       string
	lastRebootReason   string
	protocol           string
	bootTime           string
	bootTimeRetryDelay string
}

func New(opts ...Option) (*ConveyHeaderProvider, error) {
	conveyHeaderProvider := &ConveyHeaderProvider{}

	for _, opt := range opts {
		if opt != nil {
			if err := opt.apply(conveyHeaderProvider); err != nil {
				return nil, err
			}
		}
	}

	return conveyHeaderProvider, nil
}

func (c *ConveyHeaderProvider) GetConveyHeader() map[string]interface{} {
	header := make(map[string]interface{})

	for _, field := range c.fields {
		switch field {
		case Firmware:
			header[field] = c.firmware
		case Hardware:
			header[field] = c.hardware
		case Manufacturer:
			header[field] = c.manufacturer
		case SerialNumber:
			header[field] = c.serialNumber
		case LastRebootReason:
			header[field] = c.lastRebootReason
		case Protocol:
			header[field] = c.protocol
		case BootTime:
			header[field] = c.bootTime
		case BootTimeRetryDelay:
			header[field] = c.bootTimeRetryDelay
		case InterfacesAvailable: // what if we can't get interfaces available?
			names, err := c.networkService.GetInterfaceNames()
			if err != nil {
				fmt.Printf("unable to get network interfaces %s", err.Error())
				continue
			}
			header[field] = names
		default:

		}
	}

	return header
}
