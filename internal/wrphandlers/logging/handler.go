// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package loghandler

import (
	"fmt"

	"github.com/xmidt-org/wrp-go/v4"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	ErrInvalidInput = fmt.Errorf("invalid input")
)

// Handler logs information about the message being processed and sends the
// message to the next handler in the chain.
type Handler struct {
	next   wrpkit.Handler
	logger *zap.Logger
	level  zapcore.Level
}

// New creates a new Handler.  If the next handler is nil or the logger is nil,
// an error is returned.  If no level is provided, DebugLevel is used.
func New(next wrpkit.Handler, logger *zap.Logger, level ...zapcore.Level) (*Handler, error) {
	if next == nil || logger == nil {
		return nil, ErrInvalidInput
	}

	level = append(level, zapcore.DebugLevel)

	return &Handler{
		next:   next,
		logger: logger,
		level:  level[0],
	}, nil
}

// HandleWrp is called to process a message.  If the next handler fails to
// process the message, a response is sent to the source of the message.
func (h Handler) HandleWrp(msg wrp.Message) error {
	fields := []zap.Field{
		zap.String("type", msg.Type.String()),
		zap.String("source", msg.Source),
		zap.String("dest", msg.Destination),
		zap.Strings("partnerids", msg.PartnerIDs),
		zap.Int("payload_size", len(msg.Payload)),
	}

	if msg.TransactionUUID != "" {
		fields = append(fields, zap.String("transaction_uuid", msg.TransactionUUID))
	}

	h.logger.Log(h.level, "Handling message", fields...)

	return h.next.HandleWrp(msg)
}
