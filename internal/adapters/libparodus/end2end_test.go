// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package libparodus

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/pubsub"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/pull"
	"go.nanomsg.org/mangos/v3/protocol/push"
)

type mockLibParodus struct {
	assert  *assert.Assertions
	require *require.Assertions
	lock    sync.Mutex
	rx      []wrp.Message
}

func (m *mockLibParodus) Listen(ctx context.Context, url string) {
	sock, err := pull.NewSocket()
	m.require.NoError(err)
	m.require.NotNil(sock)

	err = sock.SetOption(mangos.OptionRecvDeadline, 100*time.Millisecond)
	m.require.NoError(err)

	err = sock.Listen(url)
	m.require.NoError(err)

	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			bytes, err := sock.Recv()
			if errors.Is(err, mangos.ErrRecvTimeout) {
				continue
			}
			m.require.NoError(err)

			var msg wrp.Message
			err = wrp.NewDecoderBytes(bytes, wrp.Msgpack).Decode(&msg)
			m.require.NoError(err)

			m.lock.Lock()
			m.rx = append(m.rx, msg)
			m.lock.Unlock()
		}
	}()
}

func (m *mockLibParodus) HasReceived(expected wrp.Message) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	for _, msg := range m.rx {
		if expected.MessageType() == msg.MessageType() {
			return true
		}
	}

	return false
}

func (m *mockLibParodus) AssertReceived(expected []wrp.Message) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.assert.Equal(expected, m.rx)
}

func (m *mockLibParodus) Send(url string, msg wrp.Message) error {
	sock, err := push.NewSocket()
	m.require.NoError(err)
	m.require.NotNil(sock)

	err = sock.SetOption(mangos.OptionSendDeadline, 100*time.Millisecond)
	m.require.NoError(err)

	err = sock.Dial(url)
	m.require.NoError(err)

	var buf []byte
	err = wrp.NewEncoderBytes(&buf, wrp.Msgpack).Encode(msg)
	m.require.NoError(err)

	return sock.Send(buf)
}

func (m *mockLibParodus) WaitFor(ctx context.Context, expected wrp.Message) {
	for {
		if m.HasReceived(expected) {
			break
		}

		if ctx.Err() != nil {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func TestEnd2End(t *testing.T) {
	lpURL := "tcp://127.0.0.1:9999"
	lpTestUrl := "tcp://127.0.0.1:9998"
	lpOtherUrl := "tcp://127.0.0.1:9997"

	assert := assert.New(t)
	require := require.New(t)

	self, err := wrp.ParseDeviceID("mac:112233445566")
	require.NoError(err)
	require.NotEmpty(self)

	ps, err := pubsub.New(self, pubsub.WithPublishTimeout(200*time.Millisecond))
	require.NoError(err)
	require.NotNil(ps)

	a, err := New(lpURL, ps,
		ReceiveTimeout(100*time.Millisecond),
		SendTimeout(100*time.Millisecond),
		KeepaliveInterval(100*time.Millisecond),
	)
	require.NoError(err)
	require.NotNil(a)

	mTest := mockLibParodus{
		assert:  assert,
		require: require,
	}

	// Create a second mock libparodus listener to simulate a second service.
	mOther := mockLibParodus{
		assert:  assert,
		require: require,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start the test mock libparodus listener.
	mTest.Listen(ctx, lpTestUrl)

	// Now start the adapter.
	err = a.Start()
	assert.NoError(err)

	// It's ok to start it multiple times.
	err = a.Start()
	assert.NoError(err)

	// Using the mock libparodus listener, send a service registration message.
	mTest.Send(lpURL, wrp.Message{
		Type:        wrp.ServiceRegistrationMessageType,
		URL:         lpTestUrl,
		ServiceName: "test",
	})

	// Start the second mock libparodus listener & register it.
	mOther.Listen(ctx, lpOtherUrl)
	mOther.Send(lpURL, wrp.Message{
		Type:        wrp.ServiceRegistrationMessageType,
		URL:         lpOtherUrl,
		ServiceName: "other",
	})

	// Wait for the service registration message to be received.
	mTest.WaitFor(ctx, wrp.Message{
		Type: wrp.AuthorizationMessageType,
		Status: func() *int64 {
			var s int64 = 200
			return &s
		}(),
	})

	// Wait for the service registration message to be received.
	mOther.WaitFor(ctx, wrp.Message{
		Type: wrp.AuthorizationMessageType,
		Status: func() *int64 {
			var s int64 = 200
			return &s
		}(),
	})

	// Send a message to the 'test' service.
	err = ps.HandleWrp(wrp.Message{
		Type:        wrp.SimpleEventMessageType,
		Source:      "mac:112233445566/eventer",
		Destination: "mac:112233445566/test",
	})
	assert.NoError(err)

	// Send a message to the 'other' service.
	err = ps.HandleWrp(wrp.Message{
		Type:        wrp.SimpleEventMessageType,
		Source:      "mac:112233445566/eventer",
		Destination: "mac:112233445566/other",
	})
	assert.NoError(err)

	// Send a message to an unknown service.
	err = ps.HandleWrp(wrp.Message{
		Type:        wrp.SimpleEventMessageType,
		Source:      "mac:112233445566/eventer",
		Destination: "mac:112233445566/unknown",
	})
	assert.ErrorIs(err, wrpkit.ErrNotHandled)

	// Check that the 'test' service received the message.
	mTest.WaitFor(ctx, wrp.Message{
		Type: wrp.SimpleEventMessageType,
	})

	// Check that the 'other' service received the message.
	mOther.WaitFor(ctx, wrp.Message{
		Type: wrp.SimpleEventMessageType,
	})

	// Send an invalid message from test to other.
	err = mTest.Send(lpURL, wrp.Message{
		Type:        wrp.Invalid0MessageType,
		Source:      "mac:112233445566/test",
		Destination: "mac:112233445566/other",
	})
	assert.NoError(err)

	// Send a message from test to other.
	err = mTest.Send(lpURL, wrp.Message{
		Type:        wrp.SimpleRequestResponseMessageType,
		Source:      "mac:112233445566/test",
		Destination: "mac:112233445566/other",
	})
	assert.NoError(err)

	// Ensure the test service was sent a keepalive message.
	mTest.WaitFor(ctx, wrp.Message{
		Type: wrp.ServiceAliveMessageType,
	})

	// Ensure the other service was sent a keepalive message.
	mOther.WaitFor(ctx, wrp.Message{
		Type: wrp.ServiceAliveMessageType,
	})

	// Send a message to the 'test' service.
	a.Stop()

	// It's ok to stop it multiple times.
	a.Stop()

	assert.False(mOther.HasReceived(wrp.Message{
		Type: wrp.Invalid0MessageType,
	}))

	// Now send a message to the 'other' service & get back that it isn't handled.
	err = ps.HandleWrp(wrp.Message{
		Type:        wrp.SimpleEventMessageType,
		Source:      "mac:112233445566/eventer",
		Destination: "mac:112233445566/other",
	})
	assert.ErrorIs(err, wrpkit.ErrNotHandled)
}
