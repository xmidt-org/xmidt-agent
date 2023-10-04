// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package event

import (
	"time"

	"github.com/google/uuid"
)

// CancelListenerFunc is the interface that provides a method to cancel
// a listener.
type CancelListenerFunc func()

// Fetch is the event that is sent when the credentials are fetched.
type Fetch struct {
	// The origin of the data - "fs" or "network" are the only valid values.
	Origin string

	// At holds the time when the fetch request was made.
	At time.Time

	// Duration is the time waited for the token/response.
	Duration time.Duration

	// UUID is the UUID of the request.
	UUID uuid.UUID

	// StatusCode is the status code returned from the SAT service.
	StatusCode int

	// RetryIn is the time to wait before retrying the request. Any value
	// less than or equal to zero means the server did not specify a
	// recommended retry time.
	RetryIn time.Duration

	// Expiration is the time the token expires.
	Expiration time.Time

	// Error is the error returned from the SAT service.
	Err error
}

// FetchListener is the interface that must be implemented by types that
// want to receive Fetch notifications.
type FetchListener interface {
	OnFetch(Fetch)
}

// FetchListenerFunc is a function type that implements FetchListener.
// It can be used as an adapter for functions that need to implement the
// FetchListener interface.
type FetchListenerFunc func(Fetch)

func (f FetchListenerFunc) OnFetch(e Fetch) {
	f(e)
}

// Decorate is the event that is sent when the request is decorated.
type Decorate struct {
	// Expiration is the time the token expires.
	Expiration time.Time

	// Error is the error returned from the SAT service.
	Err error
}

// DecorateListener is the interface that must be implemented by types that
// want to receive Decorate notifications.
type DecorateListener interface {
	OnDecorate(Decorate)
}

// DecorateListenerFunc is a function type that implements DecorateListener.
// It can be used as an adapter for functions that need to implement the
// DecorateListener interface.
type DecorateListenerFunc func(Decorate)

func (f DecorateListenerFunc) OnDecorate(e Decorate) {
	f(e)
}
