// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package convey

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

type ConveyHeader struct {
	Firmware string `json:"fw-name"`
	Hardware string  `json:"hw-model"`
	Manufacturer string `json:"hw-manufacturer"`
	SerialNumber string `json:"hw-serial-number"`
	LastRebootReason string `json:"hw-last-reboot-reason"`
	LastReconnectReason string `json:"webpa-last-reconnect-reason"`
	Protocol string `json:"webpa-protocol"`
	BootTime string `json:"boot-time"`
	BootTimeRetryDelay string `json:"boot-time-retry-wait"`
	InterfaceUsed string `json:"webpa-interface-used"`
	InterfacesAvailable []string `json:"interfaces-avail"`
}

type ConveyHeaderProvider struct {
	firmware string
	hardware string
	manufacturer string
	serialNumber string
	lastRebootReason string
	lastReconnectReason string
	protocol string
	bootTime string
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

func (c *ConveyHeaderProvider) GetConveyHeader() ConveyHeader {
	return ConveyHeader {
		Firmware: c.firmware,
		Hardware: c.hardware, 
		Manufacturer: c.manufacturer,
		SerialNumber: c.serialNumber,
		LastRebootReason: c.lastRebootReason,
		LastReconnectReason: c.lastReconnectReason,
		Protocol: c.protocol,
		BootTime: c.bootTime,
		BootTimeRetryDelay: c.bootTimeRetryDelay,
	}
}