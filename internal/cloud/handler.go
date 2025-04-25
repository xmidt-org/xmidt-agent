// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"github.com/xmidt-org/xmidt-agent/internal/event"
)

// Handler interface is used to handle networking calls to and from the cloud in a similar way
type Handler interface {
	// connect to cloud and start sending and receiving messages
	Start()
	// disconnect from cloud
	Stop()
	// any listener added will be called when the network channel receives a messages from the cloud
	AddMessageListener(listener event.MsgListener) event.CancelFunc
	// any listener added will be called when the network channel tries to connect
	AddConnectListener(listener event.ConnectListener) event.CancelFunc
	Name() string
}
