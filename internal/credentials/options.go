// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	iofs "io/fs"
	"net/http"
	"time"

	"github.com/xmidt-org/wrp-go/v4"
	"github.com/xmidt-org/xmidt-agent/internal/credentials/event"
	"github.com/xmidt-org/xmidt-agent/internal/fs"
)

type optionFunc func(*Credentials) error

var _ Option = optionFunc(nil)

func (f optionFunc) apply(c *Credentials) error {
	return f(c)
}

type nilOptionFunc func(*Credentials)

var _ Option = nilOptionFunc(nil)

func (f nilOptionFunc) apply(c *Credentials) error {
	f(c)
	return nil
}

// URL is the URL of the credential service.
func URL(url string) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			c.url = url
		})
}

// HTTPClient is the HTTP client used to fetch the credentials.
func HTTPClient(client *http.Client) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			if client == nil {
				client = http.DefaultClient
			}
			c.client = client
		})
}

// RefetchPercent is the percentage of the lifetime of the credentials
// that must pass before a refetch is attempted. The accepted range is 0.0 to
// 100.0. If 0.0 is specified the default is used. The default is 90.0.
func RefetchPercent(percent float64) Option {
	return optionFunc(
		func(c *Credentials) error {
			if percent < 0.0 || percent > 100.0 {
				return ErrInvalidInput
			}

			c.refetchPercent = percent

			if c.refetchPercent == 0.0 {
				c.refetchPercent = DefaultRefetchPercent
			}
			return nil
		})
}

// AssumedLifetime is the lifetime of the credentials that is assumed if the
// credentials service does not return a lifetime.  A value of zero means that
// no assumed lifetime is used.  The default is zero.
func AssumedLifetime(lifetime time.Duration) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			c.assumedLifetime = lifetime
		})
}

// IgnoreBody is a flag that indicates whether the body of the response should
// be ignored instead of examined for an expiration time.  The default is to
// examine the body.
func IgnoreBody() Option {
	return nilOptionFunc(
		func(c *Credentials) {
			c.ignoreBody = true
		})
}

// Required is a flag that indicates whether the credentials are required to
// successfully decorate a request.  The default is optional.
func Required() Option {
	return nilOptionFunc(
		func(c *Credentials) {
			c.required = true
		})
}

// LocalStorage is the local storage used to cache the credentials.
//
// The filename (and path) is relative to the provided filesystem.
func LocalStorage(fs fs.FS, filename string, perm iofs.FileMode) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			c.fs = fs
			c.filename = filename
			c.perm = perm
		})
}

// MacAddress is the MAC address of the device.
func MacAddress(macAddress wrp.DeviceID) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			c.macAddress = macAddress
		})
}

// SerialNumber is the serial number of the device.
func SerialNumber(serialNumber string) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			c.serialNumber = serialNumber
		})
}

// HardwareModel is the hardware model of the device.
func HardwareModel(hardwareModel string) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			c.hardwareModel = hardwareModel
		})
}

// HardwareManufacturer is the hardware manufacturer of the device.
func HardwareManufacturer(hardwareManufacturer string) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			c.hardwareManufacturer = hardwareManufacturer
		})
}

// FirmwareVersion is the firmware version of the device.
func FirmwareVersion(firmwareVersion string) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			c.firmwareVersion = firmwareVersion
		})
}

// LastRebootReason is the reason for the most recent reboot of the device.
func LastRebootReason(lastRebootReason string) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			c.lastRebootReason = lastRebootReason
		})
}

// XmidtProtocol is the protocol version used by the device to communicate with
// the Xmidt cluster.
func XmidtProtocol(xmidtProtocol string) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			c.xmidtProtocol = xmidtProtocol
		})
}

// BootRetryWait is the time to wait before retrying the request. Any value
// less than or equal to zero is treated as zero.
func BootRetryWait(bootRetryWait time.Duration) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			c.bootRetryWait = bootRetryWait
		})
}

// LastReconnectReason is the reason for the most recent reconnect of the
// device.  This is a dynamic value that is obtained by calling the function
// provided.
func LastReconnectReason(lastReconnectReason func() string) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			if lastReconnectReason == nil {
				lastReconnectReason = func() string { return "" }
			}
			c.lastReconnectReason = lastReconnectReason
		})
}

// PartnerID is the partner ID of the device.  This is a dynamic value that is
// obtained by calling the function provided.
func PartnerID(partnerID func() string) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			if partnerID == nil {
				partnerID = func() string { return "" }
			}
			c.partnerID = partnerID
		})
}

// NowFunc is the function used to obtain the current time.
func NowFunc(nowFunc func() time.Time) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			if nowFunc == nil {
				nowFunc = time.Now
			}
			c.nowFunc = nowFunc
		})
}

// AddFetchListener adds a listener for fetch events.  If the optional cancel
// parameter is provided, it is set to a function that can be used to cancel
// the listener.
func AddFetchListener(listener event.FetchListener, cancel ...*event.CancelListenerFunc) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			cncl := c.fetchListeners.Add(listener)
			if len(cancel) > 0 && cancel[0] != nil {
				*cancel[0] = event.CancelListenerFunc(cncl)
			}
		})
}

// AddDecorateListener adds a listener for decorate events.  If the optional
// cancel parameter is provided, it is set to a function that can be used to
// cancel the listener.
func AddDecorateListener(listener event.DecorateListener, cancel ...*event.CancelListenerFunc) Option {
	return nilOptionFunc(
		func(c *Credentials) {
			cncl := c.decorateListeners.Add(listener)
			if len(cancel) > 0 && cancel[0] != nil {
				*cancel[0] = event.CancelListenerFunc(cncl)
			}
		})
}
