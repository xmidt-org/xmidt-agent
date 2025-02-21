// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package quic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/xmidt-agent/internal/websocket/event"
)

func Test_ipMode_ToEvent(t *testing.T) {
	tests := []struct {
		description string
		m           ipMode
		want        event.IPMode
	}{
		{
			description: "ipv4",
			m:           ipv4,
			want:        event.IPv4,
		}, {
			description: "ipv6",
			m:           ipv6,
			want:        event.IPv6,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			assert.Equal(tc.want, tc.m.ToEvent())
		})
	}
}
