// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrpkit

import (
	"errors"

	"github.com/xmidt-org/wrp-go/v5"
)

var (
	ErrNotHandled = errors.New("message not handled")
)

// Handler interface is used to handle wrp messages in the system in a
// consistent way.
type Handler interface {
	// HandleWrp is called whenever a message is received that matches the
	// criteria associated with the handler.
	//
	// Unless the error of ErrNotHandled is returned, the handler is
	// considered to have consumed the message.  It is up to the handler to
	// perform any responses or further actions.
	HandleWrp(wrp.Message) error
}

// HandlerFunc is an adapter to allow the use of ordinary functions as handlers.
type HandlerFunc func(wrp.Message) error

func (f HandlerFunc) HandleWrp(msg wrp.Message) error {
	return f(msg)
}

var _ Handler = HandlerFunc(nil)
