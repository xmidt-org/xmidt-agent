// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package event

import (
	"time"
)

// Fetch is an event that is emitted when a TXT record fetch is attempted.
type Fetch struct {
	// FQDN is the fully qualified domain name of the TXT record.
	FQDN string

	// Server is the DNS server that was queried.
	Server string

	// Found indicates whether the TXT record was found.
	Found bool

	// Timeout indicates whether the query timed out.
	Timeout bool

	// PriorExpiration is the expiration time of the previous TXT record.
	PriorExpiration time.Time

	// Expiration is the expiration time of the TXT record.
	Expiration time.Time

	// TemporaryErr indicates whether a temporary error occurred during the query.
	TemporaryErr bool

	// The endpoint that was found in the TXT record.
	Endpoint string

	// Payload is the payload of the TXT record.
	Payload []byte

	// Err indicates whether an error occurred during the query.
	Err error
}

// FetchListener is a sink for registration events.
type FetchListener interface {
	OnFetchEvent(Fetch)
}

// FetchListenerFunc is a function type that implements FetchListener.
type FetchListenerFunc func(Fetch)

func (f FetchListenerFunc) OnFetchEvent(fe Fetch) {
	f(fe)
}
