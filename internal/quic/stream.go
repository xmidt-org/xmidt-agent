// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package quic

import (
	"context"
	"time"

	"github.com/quic-go/quic-go"
)

type Stream interface {
	StreamID() quic.StreamID
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	CancelRead(errorCode quic.StreamErrorCode)
	Context() context.Context
	Close() error
	SetReadDeadline(t time.Time) error
	SetDeadline(t time.Time) error
}
