// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package credentials

import "fmt"

func urlVador() Option {
	return optionFunc(
		func(c *Credentials) error {
			if c.url == "" {
				return fmt.Errorf("%w URL is missing", ErrInvalidInput)
			}
			return nil
		})
}

func macAddressVador() Option {
	return optionFunc(
		func(c *Credentials) error {
			if len(c.macAddress) == 0 {
				return fmt.Errorf("%w mac address is missing", ErrInvalidInput)
			}
			return nil
		})
}

func serialNumberVador() Option {
	return optionFunc(
		func(c *Credentials) error {
			if c.serialNumber == "" {
				return fmt.Errorf("%w serial number is missing", ErrInvalidInput)
			}
			return nil
		})
}

func hardwareModelVador() Option {
	return optionFunc(
		func(c *Credentials) error {
			if c.hardwareModel == "" {
				return fmt.Errorf("%w hardware model is missing", ErrInvalidInput)
			}
			return nil
		})
}

func hardwareManufacturerVador() Option {
	return optionFunc(
		func(c *Credentials) error {
			if c.hardwareManufacturer == "" {
				return fmt.Errorf("%w hardware manufacturer is missing", ErrInvalidInput)
			}
			return nil
		})
}

func firmwareVersionVador() Option {
	return optionFunc(
		func(c *Credentials) error {
			if c.firmwareVersion == "" {
				return fmt.Errorf("%w firmware version is missing", ErrInvalidInput)
			}
			return nil
		})
}

func lastRebootReasonVador() Option {
	return optionFunc(
		func(c *Credentials) error {
			if c.lastRebootReason == "" {
				return fmt.Errorf("%w last reboot reason is missing", ErrInvalidInput)
			}
			return nil
		})
}

func xmidtProtocolVador() Option {
	return optionFunc(
		func(c *Credentials) error {
			if c.xmidtProtocol == "" {
				return fmt.Errorf("%w xmidt protocol is missing", ErrInvalidInput)
			}
			return nil
		})
}

func bootRetryWaitVador() Option {
	return optionFunc(
		func(c *Credentials) error {
			if c.bootRetryWait == 0 {
				return fmt.Errorf("%w boot retry wait is missing", ErrInvalidInput)
			}
			return nil
		})
}
