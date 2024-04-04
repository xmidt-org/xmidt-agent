// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"context"
	"fmt"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/websocket"
)

var (
	ErrInvalidInput = fmt.Errorf("invalid input")
)

// Handler sends a response when a message is required to have a response.
type Handler struct {
	ws *websocket.Websocket
}

// New creates a new instance of the Handler struct.  The parameter ws is the
// websocket that will used to send to a response.
func New(ws *websocket.Websocket) (*Handler, error) {
	if ws == nil {
		return nil, ErrInvalidInput
	}

	return &Handler{
		ws: ws,
	}, nil
}

// HandleWrp is called to process a message.
func (h Handler) HandleWrp(msg wrp.Message) error {
	if !msg.Type.RequiresTransaction() {
		return nil
	}

	return h.ws.Send(context.Background(), msg)
}
