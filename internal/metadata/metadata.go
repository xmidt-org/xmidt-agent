// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/net"
)

type Option interface {
	apply(*MetadataProvider) error
}

type optionFunc func(*MetadataProvider) error

func (f optionFunc) apply(c *MetadataProvider) error {
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

type MetadataProvider struct {
	networkService     net.NetworkServicer
	fields             []string
	firmware           string
	hardware           string
	manufacturer       string
	serialNumber       string
	lastRebootReason   string
	protocol           string
	bootTime           string
	bootTimeRetryDelay string
	interfaceUsed      *InterfaceUsedProvider
}

func New(opts ...Option) (*MetadataProvider, error) {
	metadataProvider := &MetadataProvider{}

	for _, opt := range opts {
		if opt != nil {
			if err := opt.apply(metadataProvider); err != nil {
				return nil, err
			}
		}
	}

	return metadataProvider, nil
}

func (c *MetadataProvider) GetMetadata() map[string]interface{} {
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
		case InterfaceUsed:
			header[field] = c.interfaceUsed.GetInterfaceUsed()
		case InterfacesAvailable: // what if we can't get interfaces available?
			names, err := c.networkService.GetInterfaceNames()
			if err != nil {
				// The err itself is ignored. Log this somewhere tho
				continue
			}
			header[field] = strings.Join(names, ",")
		default:

		}
	}

	return header
}

func (c *MetadataProvider) Decorate(headers http.Header) error {
	header := c.GetMetadata()
	headerBytes, err := json.Marshal(header)
	if err != nil {
		// TODO use eventor to log
		return fmt.Errorf("error marshaling convey header: %w", err)
	}

	headers.Set(HeaderName, string(headerBytes))

	return nil
}

func (c *MetadataProvider) DecorateMsg(msg *wrp.Message) error {
	header := c.GetMetadata()

	// test this
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]string)
	}

	for key, value := range header {
		if value != nil {
			msg.Metadata[key] = value.(string)
		}
	}

	return nil
}
