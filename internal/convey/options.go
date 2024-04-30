// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package convey

import (
	"errors"
)

var (
	ErrInvalidInput = errors.New("invalid input")
)

func serialNumber(serialNumber string) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			c.serialNumber = serialNumber
			return nil
		})
}

func hardwareModel(hardwareModel string) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			c.hardware = hardwareModel
			return nil
		})
}

func manufacturer(manufacturer string) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			c.manufacturer = manufacturer
			return nil
		})
}

func lastRebootReason(lastRebootReason string) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			c.lastRebootReason = lastRebootReason
			return nil
		})
}

func lastReconnectReason(lastReconnectReason string) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			c.lastReconnectReason = lastReconnectReason
			return nil
		})
}

func protocol(protocol string) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			c.protocol = protocol
			return nil
		})
}

func bootTime(bootTime string) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			c.bootTime = bootTime
			return nil
		})
}

func bootTimeRetryDelay(bootTimeRetryDelay string) Option {
	return optionFunc(
		func(c *ConveyHeaderProvider) error {
			c.bootTimeRetryDelay = bootTimeRetryDelay
			return nil
		})
}
