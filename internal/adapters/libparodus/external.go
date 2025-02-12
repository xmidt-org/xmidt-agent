// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package libparodus

import (
	"context"
	"sync"
	"time"

	"github.com/xmidt-org/wrp-go/v4"
	"github.com/xmidt-org/xmidt-agent/internal/pubsub"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/push"
)

var (
	// authAcceptedMsg is a WRP message indicating that the external service
	// has accepted the authorization request.  This message never changes,
	// so it is safe to encode it once and reuse it.
	authAcceptedMsg = wrp.MustEncode(
		&wrp.Message{
			Type: wrp.AuthorizationMessageType,
			Status: func() *int64 {
				s := int64(200)
				return &s
			}(),
		},
		wrp.Msgpack)

	// serviceAliveMsg is a WRP message indicating that the pipe to the external
	// service is still alive.  This message never changes, so it is safe to
	// encode it once and reuse it.
	serviceAliveMsg = wrp.MustEncode(
		&wrp.Message{
			Type: wrp.ServiceAliveMessageType,
		},
		wrp.Msgpack)
)

// external is a struct representing a subscription to an external service.
type external struct {
	name              string
	heartbeatInterval time.Duration
	terminate         func()

	// Everything below is private to the sub
	lock sync.Mutex
	sock protocol.Socket
}

var _ wrpkit.Handler = (*external)(nil)

// newExternal creates a new subscription to an external service.  Unlike a standard
// 'New()' function, this function also connects to the external service.
func newExternal(ctx context.Context,
	name, url string,
	heartbeatInterval, sendTimeout time.Duration,
	ps *pubsub.PubSub,
	terminate func()) (*external, error) {

	ex := external{
		name:              name,
		heartbeatInterval: heartbeatInterval,
	}

	ex.lock.Lock()
	defer ex.lock.Unlock()

	// If the socket is already open, close it and open a new one.
	if ex.sock != nil {
		_ = ex.sock.Close()
		ex.sock = nil
	}

	sock, err := push.NewSocket()
	if err != nil {
		return nil, err
	}

	// Set the send timeout to the configured value.  The other methods of
	// setting the timeout are not supported by the mangos library.
	if err = sock.SetOption(mangos.OptionSendDeadline, sendTimeout); err != nil {
		return nil, err
	}

	// Set the write queue length to 1.  This is the only way to ensure that
	// message delivery faiures are detected.
	if err = sock.SetOption(mangos.OptionWriteQLen, 1); err != nil {
		return nil, err
	}

	err = sock.Dial(url)
	if err != nil {
		_ = sock.Close()
		terminate()
		return nil, err
	}

	ex.sock = sock

	ctx, cancel := context.WithCancel(ctx)

	psCancel, err := ps.SubscribeService(ex.name, &ex)
	if err != nil {
		cancel()
		_ = sock.Close()
		terminate()
		return nil, err
	}

	ex.terminate = func() {
		psCancel()
		cancel()
		_ = sock.Close()
		terminate()
	}

	go ex.keepalive(ctx)

	return &ex, nil
}

// HandleWrp sends a WRP message to the external service.
func (s *external) HandleWrp(msg wrp.Message) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.sock == nil {
		return wrpkit.ErrNotHandled
	}

	var buf []byte
	if err := wrp.NewEncoderBytes(&buf, wrp.Msgpack).Encode(msg); err != nil {
		return err
	}

	return s.sock.Send(buf)
}

// keepalive sends a keepalive message to the external service.  At some point
// the external service will stop sending heartbeats, and the subscription will
// be canceled.  Do not call this except from newExternal().
func (s *external) keepalive(ctx context.Context) {
	s.lock.Lock()
	err := s.sock.Send(authAcceptedMsg)
	s.lock.Unlock()
	if err == nil {
		for {
			select {
			case <-ctx.Done(): //context canceled
				break
			case <-time.After(s.heartbeatInterval):
			}

			s.lock.Lock()
			err := s.sock.Send(serviceAliveMsg)
			s.lock.Unlock()

			if err != nil {
				// The heartbeat failed.  Cancel the subscription & exit.
				break
			}
		}
	}

	s.cancel()
}

// cancel cancels the subscription to the external service.
func (s *external) cancel() {
	s.terminate()
}
