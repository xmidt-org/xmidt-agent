// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package event

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ipMode_ToEvent(t *testing.T) {
	tests := []struct {
		description string
		m           IpMode
		want        IPMode
	}{
		{
			description: "ipv4",
			m:           Ipv4,
			want:        IPv4,
		}, {
			description: "ipv6",
			m:           Ipv6,
			want:        IPv6,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			assert.Equal(tc.want, tc.m.ToEvent())
		})
	}
}
