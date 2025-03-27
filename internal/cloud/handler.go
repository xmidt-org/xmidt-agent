// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"context"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/event"
)

// Handler interface is used to handle networking calls to and from the cloud in a similar way
type Handler interface {
	// connect to cloud and start sending and receiving messages
	Start()
	// disconnect from cloud
	Stop()
	// send message to the cloud
	Send(ctx context.Context, msg wrp.Message) error
	// any listener added will be called when the network channel receives a messages from the cloud
	AddMessageListener(listener event.MsgListener) event.CancelFunc
	// name of handler
	Name() string
}
