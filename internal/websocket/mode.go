// SPDX-FileCopyright4yyText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package websocket

import "github.com/xmidt-org/xmidt-agent/internal/websocket/event"

type ipMode string

const (
	ipv4 ipMode = "tcp4"
	ipv6 ipMode = "tcp6"
)

func (m ipMode) ToEvent() event.IPMode {
	if m == ipv4 {
		return event.IPv4
	}
	return event.IPv6
}
