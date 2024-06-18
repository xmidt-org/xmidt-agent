// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package missing

import (
	"errors"
	"fmt"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

var (
	ErrInvalidInput = fmt.Errorf("invalid input")
)

const (
	// statusCode is the status code to return when a message is missing a handler.
	statusCode = 531
)

// Handler sends a response when a message is required to have a response, but
// was not handled by the next handler in the chain.
type Handler struct {
	next   wrpkit.Handler
	egress wrpkit.Handler
	source string
}

// New creates a new instance of the Handler struct.  The parameter next is the
// handler that will be called and monitored for errors.  The parameter egress is
// the handler that will be called to send the response if/when the next handler
// fails to handle the message.  The parameter source is the source to use in
// the response message.
func New(next, egress wrpkit.Handler, source string) (*Handler, error) {
	if next == nil || egress == nil || source == "" {
		return nil, ErrInvalidInput
	}

	return &Handler{
		next:   next,
		egress: egress,
		source: source,
	}, nil
}

// HandleWrp is called to process a message.  If the next handler fails to
// process the message, a response is sent to the source of the message.
func (h Handler) HandleWrp(msg wrp.Message) error {
	err := h.next.HandleWrp(msg)
	if err == nil {
		return nil
	}

	if !msg.Type.RequiresTransaction() {
		return err
	}

	// If the error is not ErrNotHandled, return the error.
	if !errors.Is(err, wrpkit.ErrNotHandled) {
		return err
	}

	// Consume the error since we are handling it here.
	err = nil

	// At this point, we know that a response is required, but the next handler
	// failed to process the message, or didn't have a handler for it.
	response := msg
	response.Destination = msg.Source
	response.Source = h.source
	response.ContentType = "application/json"

	code := int64(statusCode)
	response.Status = &code
	response.Payload = []byte(fmt.Sprintf("{statusCode: %d}", code))

	sendErr := h.egress.HandleWrp(response)

	return errors.Join(err, sendErr)
}
