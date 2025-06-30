// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package metadata

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
		func(c *MetadataProvider) error {
			if networkService == nil {
				return fmt.Errorf("%w: nil networkService", ErrInvalidInput)
			}
			c.networkService = networkService
			return nil
		})
}

func FieldsOpt(fields []string) Option {
	return optionFunc(
		func(c *MetadataProvider) error {
			for _, field := range fields {
				valid := false
				for _, validField := range validFields {
					if field == validField {
						valid = true
					}
				}
				if !valid {
					return fmt.Errorf("%w: invalid metadata field", ErrInvalidInput)
				}
			}
			c.fields = fields
			return nil
		})
}

func SerialNumberOpt(serialNumber string) Option {
	return optionFunc(
		func(c *MetadataProvider) error {
			c.serialNumber = serialNumber
			return nil
		})
}

func HardwareModelOpt(hardwareModel string) Option {
	return optionFunc(
		func(c *MetadataProvider) error {
			c.hardware = hardwareModel
			return nil
		})
}

func FirmwareOpt(firmware string) Option {
	return optionFunc(
		func(c *MetadataProvider) error {
			c.firmware = firmware
			return nil
		})
}

func ManufacturerOpt(manufacturer string) Option {
	return optionFunc(
		func(c *MetadataProvider) error {
			c.manufacturer = manufacturer
			return nil
		})
}

func LastRebootReasonOpt(lastRebootReason string) Option {
	return optionFunc(
		func(c *MetadataProvider) error {
			c.lastRebootReason = lastRebootReason
			return nil
		})
}

func XmidtProtocolOpt(protocol string) Option {
	return optionFunc(
		func(c *MetadataProvider) error {
			c.protocol = protocol
			return nil
		})
}

func BootTimeOpt(bootTime string) Option {
	return optionFunc(
		func(c *MetadataProvider) error {
			c.bootTime = bootTime
			return nil
		})
}

func BootRetryWaitOpt(bootTimeRetryDelay time.Duration) Option {
	return optionFunc(
		func(c *MetadataProvider) error {
			c.bootTimeRetryDelay = fmt.Sprint(bootTimeRetryDelay.Seconds())
			return nil
		})
}

// if we want websocket to populate this value, pass in InterfaceUsedProvider instead
func InterfaceUsedOpt(interfaceUsed string) Option {
	return optionFunc(
		func(c *MetadataProvider) error {
			c.interfaceUsed = interfaceUsed
			return nil
		})
}

// the identity metadata is usually transmitted to the cloud via the convey header on connect. optionally
// append the identity metadata to every wrp message. (temporary workaround for home cloud issues)
func AppendToMsg(appendToMsg bool) Option {
	return optionFunc(
		func(c *MetadataProvider) error {
			c.appendToMsg = appendToMsg
			return nil
		})
}
