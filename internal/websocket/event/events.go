// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package event

import (
	"fmt"
	"strings"
	"time"

	"github.com/xmidt-org/wrp-go/v4"
)

// HeartbeatType is the type of heartbeat that occurred.
type HeartbeatType int

const (
	PING HeartbeatType = iota
	PONG
)

type IPMode string

const (
	IPv4 IPMode = "IPv4"
	IPv6 IPMode = "IPv6"
)

// CancelFunc is the interface that provides a method to cancel a listener.
type CancelFunc func()

type Connect struct {
	// Started holds the time when the connection attempt was started.
	Started time.Time

	// At holds the time when the connection was made/errored out.
	At time.Time

	// Mode is the IP mode used to connect.
	Mode IPMode

	// RetryingAt is the time when the next connection attempt will be made.
	RetryingAt time.Time

	// Error is the error returned from the attempt to connect.
	Err error
}

func (c Connect) String() string {
	var buf strings.Builder
	buf.WriteString("Connect{\n")
	fmt.Fprintf(&buf, "  Started:    %s\n", c.Started.Format(time.RFC3339Nano))
	fmt.Fprintf(&buf, "  At:         %s (%s)\n", c.At.Format(time.RFC3339Nano), c.At.Sub(c.Started))
	fmt.Fprintf(&buf, "  Mode:       %s\n", string(c.Mode))
	if !c.RetryingAt.IsZero() {
		fmt.Fprintf(&buf, "  RetryingAt: %s\n", c.RetryingAt.Format(time.RFC3339Nano))
	}
	if c.Err != nil {
		fmt.Fprintf(&buf, "  Err:        %s\n", c.Err)
	}
	buf.WriteString("}")

	return buf.String()
}

// ConnectListener is the interface that must be implemented by types that
// want to receive Connect notifications.
type ConnectListener interface {
	OnConnect(Connect)
}

// ConnectListenerFunc is a function type that implements ConnectListener.
// It can be used as an adapter for functions that need to implement the
// ConnectListener interface.
type ConnectListenerFunc func(Connect)

func (f ConnectListenerFunc) OnConnect(c Connect) {
	f(c)
}

// Heartbeat is the event that is sent when the heartbeat PING is received and
// the PONG is sent.
type Heartbeat struct {
	// At holds the time when the heartbeat occurred.
	At time.Time

	// Type is the type of heartbeat that occurred.
	Type HeartbeatType
}

// HeartbeatListener is the interface that must be implemented by types that
// want to receive Heartbeat notifications.
type HeartbeatListener interface {
	OnHeartbeat(Heartbeat)
}

// HeartbeatListenerFunc is a function type that implements HeartbeatListener.
// It can be used as an adapter for functions that need to implement the
// HeartbeatListener interface.
type HeartbeatListenerFunc func(Heartbeat)

func (f HeartbeatListenerFunc) OnHeartbeat(h Heartbeat) {
	f(h)
}

// Disconnect is the event that is sent when the connection is closed.
type Disconnect struct {
	// At holds the time when the connection was closed.
	At time.Time

	// Error is the error returned from the disconnection.
	Err error
}

// DisconnectListener is the interface that must be implemented by types that
// want to receive Disconnect notifications.
type DisconnectListener interface {
	OnDisconnect(Disconnect)
}

// DisconnectListenerFunc is a function type that implements DisconnectListener.
// It can be used as an adapter for functions that need to implement the
// DisconnectListener interface.
type DisconnectListenerFunc func(Disconnect)

func (f DisconnectListenerFunc) OnDisconnect(d Disconnect) {
	f(d)
}

// MsgListener is the interface that must be implemented by types that want
// to receive wrp.Message notifications from the websocket.
type MsgListener interface {
	OnMessage(wrp.Message)
}

// MsgListenerFunc is a function type that implements MsgListener.  It can be
// used as an adapter for functions that need to implement the MsgListener
// interface.
type MsgListenerFunc func(wrp.Message)

func (f MsgListenerFunc) OnMessage(m wrp.Message) {
	f(m)
}
