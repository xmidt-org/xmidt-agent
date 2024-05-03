// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package convey

import (
	"errors"
	"fmt"
	"time"

	"github.com/xmidt-org/xmidt-agent/internal/net"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	validFields     = []string{Firmware, Hardware, SerialNumber, Manufacturer, LastRebootReason, Protocol, BootTime, BootTimeRetryDelay, InterfaceUsed, InterfacesAvailable}
)

func NetworkServiceOpt(networkService net.NetworkServicer) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			if networkService == nil {
				fmt.Printf("nil networkService")
				return ErrInvalidInput
			}
			c.networkService = networkService
			return nil
		})
}

func FieldsOpt(fields []string) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			for _, field := range fields {
				valid := false
				for _, validField := range validFields {
					if field == validField {
						valid = true
					}
				}
				if !valid {
					fmt.Printf("invalid metadata field %s", field)
					return ErrInvalidInput
				}
			}
			c.fields = fields
			return nil
		})
}

func SerialNumberOpt(serialNumber string) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			c.serialNumber = serialNumber
			return nil
		})
}

func HardwareModelOpt(hardwareModel string) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			c.hardware = hardwareModel
			return nil
		})
}

func FirmwareOpt(firmware string) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			c.firmware = firmware
			return nil
		})
}

func ManufacturerOpt(manufacturer string) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			c.manufacturer = manufacturer
			return nil
		})
}

func LastRebootReasonOpt(lastRebootReason string) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			c.lastRebootReason = lastRebootReason
			return nil
		})
}

func XmidtProtocolOpt(protocol string) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			c.protocol = protocol
			return nil
		})
}

func BootTimeOpt(bootTime string) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			c.bootTime = bootTime
			return nil
		})
}

func BootRetryWaitOpt(bootTimeRetryDelay time.Duration) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			c.bootTimeRetryDelay = fmt.Sprint(bootTimeRetryDelay.Seconds())
			return nil
		})
}
