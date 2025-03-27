// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package event

type IpMode string

const (
	Ipv4 IpMode = "tcp4"
	Ipv6 IpMode = "tcp6"
)

func (m IpMode) ToEvent() IPMode {
	if m == Ipv4 {
		return IPv4
	}
	return IPv6
}
