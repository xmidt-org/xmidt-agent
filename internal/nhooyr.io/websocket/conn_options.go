// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"errors"
	"time"
)

var (
	ErrMisconfiguredWS = errors.New("misconfigured WS")
)

// Option is a functional option type for WS.
type ConnOption interface {
	apply(*Conn)
}

type ConnOptionFunc func(*Conn)

func (f ConnOptionFunc) apply(c *Conn) {
	f(c)
}

// InactivityTimeout sets inactivity timeout for the WS connection.
// Nonpositive InactivityTimeout will default to a 1 minute timeout.
func InactivityTimeout(d time.Duration) ConnOption {
	return ConnOptionFunc(
		func(c *Conn) {
			c.inactivityTimeout = d
		})
}
