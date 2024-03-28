// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package libparodus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/pubsub"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/push"
)

type sub struct {
	name              string
	heartbeatInterval time.Duration
	terminate         func()

	// Everything below is private to the sub
	lock sync.Mutex
	sock protocol.Socket
}

var _ wrpkit.Handler = (*sub)(nil)

func newSub(ctx context.Context,
	name, url string,
	heartbeatInterval, sendTimeout time.Duration,
	ps *pubsub.PubSub,
	terminate func()) (*sub, error) {

	s := sub{
		name:              name,
		heartbeatInterval: heartbeatInterval,
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	// If the socket is already open, close it and open a new one.
	if s.sock != nil {
		_ = s.sock.Close()
		s.sock = nil
	}

	sock, err := push.NewSocket()
	if err != nil {
		return nil, err
	}

	// Set the send timeout to the configured value.  The other methods of
	// setting the timeout are not supported by the mangos library.
	//if err = sock.SetOption(mangos.OptionSendDeadline, sendTimeout); err != nil {
	//return nil, err
	//}

	// Set the write queue length to 1.  This is the only way to ensure that
	// message delivery faiures are detected.
	/*
		if err = sock.SetOption(mangos.OptionWriteQLen, 1); err != nil {
			return nil, err
		}
	*/

	s.terminate = func() {
		_ = sock.Close()
		terminate()
	}

	err = sock.Dial(url)
	if err != nil {
		s.terminate()
		return nil, err
	}

	s.sock = sock

	ctx, cancel := context.WithCancel(ctx)
	s.terminate = func() {
		cancel()
		_ = sock.Close()
		terminate()
	}

	psCancel, err := ps.SubscribeService(s.name, &s)
	if err != nil {
		s.terminate()
		return nil, err
	}
	s.terminate = func() {
		psCancel()
		cancel()
		_ = sock.Close()
		terminate()
	}

	go s.heartbeat(ctx)

	return &s, nil
}

func (s *sub) HandleWrp(msg wrp.Message) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.sock == nil {
		return nil
	}

	var buf []byte
	err := wrp.NewEncoderBytes(&buf, wrp.Msgpack).Encode(msg)
	if err != nil {
		return err
	}

	return s.sock.Send(buf)
}

func (s *sub) heartbeat(ctx context.Context) {
	auth := wrp.Message{
		Type: wrp.AuthorizationMessageType,
	}

	keepalive := wrp.Message{
		Type: wrp.ServiceAliveMessageType,
	}

	var authBuf, keepaliveBuf []byte
	_ = wrp.NewEncoderBytes(&authBuf, wrp.Msgpack).Encode(&auth)
	_ = wrp.NewEncoderBytes(&keepaliveBuf, wrp.Msgpack).Encode(&keepalive)

	//buf = []byte{0x81, 0xa8, 'm', 's', 'g', '_', 't', 'y', 'p', 'e', 0x0a}

	fmt.Println("Sending auth")
	s.lock.Lock()
	err := s.sock.Send(authBuf)
	s.lock.Unlock()
	if err != nil {
		fmt.Println("Auth failed")
		s.cancel()
		return
	}
	fmt.Println("Auth sent")

	for {
		if ctx.Err() != nil {
			return
		}

		time.Sleep(s.heartbeatInterval)

		fmt.Println("Sending heartbeat for service: ", s.name)
		s.lock.Lock()
		err := s.sock.Send(keepaliveBuf)
		s.lock.Unlock()

		if err != nil {
			// The heartbeat failed.  Cancel the subscription.
			s.cancel()
			fmt.Println("Heartbeat for service failed: ", s.name)
		} else {
			fmt.Println("Heartbeat for service sent: ", s.name)
		}
	}
}

func (s *sub) cancel() {
	s.terminate()
}
