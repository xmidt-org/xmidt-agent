// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package storage

import "time"

//go:generate go install github.com/tinylib/msgp@latest
//go:generate msgp -io=false -tests=false
//msgp:newtime

// Info is the token returned from the server as well as the expiration
// time.
type Info struct {
	Token     string    `msg:"token"`
	ExpiresAt time.Time `msg:"expires_at"`
}
