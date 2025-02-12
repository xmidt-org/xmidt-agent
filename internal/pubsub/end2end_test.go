// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package pubsub_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/wrp-go/v4"
	"github.com/xmidt-org/xmidt-agent/internal/pubsub"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

type msgWithExpectations struct {
	msg       wrp.Message
	expectErr error
}

var messages = []msgWithExpectations{
	{
		msg: wrp.Message{
			Type:        wrp.SimpleEventMessageType,
			Source:      "self:/service/ignored",
			Destination: "event:event_1/ignored",
		},
	}, {
		msg: wrp.Message{
			Type:        wrp.SimpleEventMessageType,
			Source:      "self:/service/ignored",
			Destination: "event:event_2/ignored",
		},
	}, {
		msg: wrp.Message{
			Type:        wrp.SimpleRequestResponseMessageType,
			Source:      "dns:tr1d1um.example.com/service/ignored",
			Destination: "mac:112233445566/service/ignored",
		},
	}, {
		msg: wrp.Message{
			Type:        wrp.SimpleRequestResponseMessageType,
			Source:      "mac:112233445566/service/ignored",
			Destination: "dns:tr1d1um.example.com/service/ignored",
		},
	}, {
		// invalid message - no src
		msg: wrp.Message{
			Type:        wrp.SimpleRequestResponseMessageType,
			Destination: "dns:tr1d1um.example.com/service/ignored",
		},
		expectErr: wrp.ErrorInvalidLocator,
	}, {
		// invalid message - no dest
		msg: wrp.Message{
			Type:   wrp.SimpleRequestResponseMessageType,
			Source: "mac:112233445566/service/ignored",
		},
		expectErr: wrp.ErrorInvalidLocator,
	}, {
		// invalid message - invalid msg type (empty)
		msg: wrp.Message{
			Source:      "mac:112233445566/service/ignored",
			Destination: "dns:tr1d1um.example.com/service/ignored",
		},
		expectErr: wrp.ErrInvalidMessageType,
	}, {
		// invalid message - a string field is not valid UTF-8
		msg: wrp.Message{
			Type:        wrp.SimpleRequestResponseMessageType,
			Source:      "self:/service/ignored",
			Destination: "dns:tr1d1um.example.com/service/ignored",
			PartnerIDs:  []string{string([]byte{0xbf})},
			ContentType: string([]byte{0xbf}),
		},
		expectErr: wrp.ErrNotUTF8,
	}, {
		// no handlers for this message
		msg: wrp.Message{
			Type:        wrp.SimpleEventMessageType,
			Source:      "self:/ignore-me",
			Destination: "self:/ignored-service/ignored",
		},
		expectErr: wrpkit.ErrNotHandled,
	},
}

type mockHandler struct {
	lock   sync.Mutex
	wg     *sync.WaitGroup
	name   string
	calls  int
	dests  []wrp.Locator
	expect []wrp.Locator
	rv     []error
}

func (h *mockHandler) WG(wg *sync.WaitGroup) {
	h.wg = wg
	h.wg.Add(len(h.expect))
	//fmt.Printf("%s adding %d to the wait group\n", h.name, len(h.expect))
}

func (h *mockHandler) HandleWrp(msg wrp.Message) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	calls := h.calls
	h.calls++

	dest, _ := wrp.ParseLocator(msg.Destination)
	h.dests = append(h.dests, dest)
	h.wg.Done()
	//fmt.Printf("%s done\n", h.name)
	if calls < len(h.rv) {
		return h.rv[calls]
	}

	return nil
}

func (h *mockHandler) assert(a *assert.Assertions) {
	h.lock.Lock()
	defer h.lock.Unlock()

	if !a.Equal(len(h.expect), h.calls, "handler %s calls mismatch", h.name) {
		return
	}

	for _, expected := range h.expect {
		var found bool
		for j, d := range h.dests {
			if expected.Scheme == d.Scheme &&
				expected.Authority == d.Authority &&
				expected.Service == d.Service &&
				expected.Ignored == d.Ignored {
				found = true
				h.dests[j].Scheme = ""
				break
			}
		}
		if !found {
			a.Fail("dest not found", "handler: %s expected: %s", h.name, expected.String())
		}
	}
}

func TestEndToEnd(t *testing.T) {
	id := wrp.DeviceID("mac:112233445566")

	var wg sync.WaitGroup

	allEventListener := &mockHandler{
		name: "allEventListener",
		expect: []wrp.Locator{
			{Scheme: "event", Authority: "event_1", Ignored: "/ignored"},
			{Scheme: "event", Authority: "event_2", Ignored: "/ignored"},
		},
	}
	allEventListener.WG(&wg)

	singleEventListener := &mockHandler{
		name: "singleEventListener",
		expect: []wrp.Locator{
			{Scheme: "event", Authority: "event_2", Ignored: "/ignored"},
		},
	}
	singleEventListener.WG(&wg)

	singleServiceListener := &mockHandler{
		name: "singleServiceListener",
		expect: []wrp.Locator{
			{Scheme: "mac", Authority: "112233445566", Service: "service", Ignored: "/ignored"},
		},
	}
	singleServiceListener.WG(&wg)

	egressListener := &mockHandler{
		name: "egressListener",
		expect: []wrp.Locator{
			{Scheme: "event", Authority: "event_1", Ignored: "/ignored"},
			{Scheme: "event", Authority: "event_2", Ignored: "/ignored"},
			{Scheme: "dns", Authority: "tr1d1um.example.com", Service: "service", Ignored: "/ignored"},
		},
	}
	egressListener.WG(&wg)

	assert := assert.New(t)
	require := require.New(t)

	transIdValidator := wrpkit.HandlerFunc(
		func(msg wrp.Message) error {
			if msg.TransactionUUID == "" {
				assert.Fail("transaction UUID is empty")
			}
			return wrpkit.ErrNotHandled
		})

	noTransIdValidator := wrpkit.HandlerFunc(
		func(msg wrp.Message) error {
			src, err := wrp.ParseLocator(msg.Source)
			assert.NoError(err)

			if src.Service == "ignore-me" {
				return wrpkit.ErrNotHandled
			}

			if msg.TransactionUUID != "" {
				assert.Fail("transaction UUID is not empty")
			}
			return wrpkit.ErrNotHandled
		})

	var allCancel, singleCancel, serviceCancel, egressCancel pubsub.CancelFunc
	ps, err := pubsub.New(id,
		pubsub.WithEgressHandler(egressListener, &egressCancel),
		pubsub.WithEventHandler("*", allEventListener, &allCancel),
		pubsub.WithEventHandler("event_2", singleEventListener, &singleCancel),
		pubsub.WithServiceHandler("service", singleServiceListener, &serviceCancel),

		pubsub.WithEgressHandler(transIdValidator),
		pubsub.WithServiceHandler("*", noTransIdValidator),
		pubsub.Normify(
			wrp.ValidateMessageType(),
			wrp.EnsureTransactionUUID(),
			wrp.ValidateOnlyUTF8Strings(),
		),
		pubsub.WithPublishTimeout(200*time.Millisecond),
	)

	require.NoError(err)
	require.NotNil(ps)
	require.NotNil(allCancel)
	require.NotNil(singleCancel)
	require.NotNil(serviceCancel)
	require.NotNil(egressCancel)

	for _, m := range messages {
		msg := m.msg
		err := ps.HandleWrp(msg)

		if m.expectErr != nil {
			assert.ErrorIs(err, m.expectErr)
		} else {
			assert.NoError(err)
		}
	}

	wg.Wait()

	allEventListener.assert(assert)
	singleEventListener.assert(assert)
	singleServiceListener.assert(assert)
	egressListener.assert(assert)
}

func TestTimeout(t *testing.T) {
	id := wrp.DeviceID("mac:112233445566")

	assert := assert.New(t)
	require := require.New(t)

	timeoutHandler := wrpkit.HandlerFunc(
		func(msg wrp.Message) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		})

	ps, err := pubsub.New(id,
		pubsub.WithPublishTimeout(50*time.Millisecond),
		pubsub.WithEgressHandler(timeoutHandler),
	)

	require.NoError(err)
	require.NotNil(ps)

	msg := wrp.Message{
		Type:        wrp.SimpleEventMessageType,
		Source:      "self:/service/ignored",
		Destination: "event:event_1/ignored",
	}

	err = ps.HandleWrp(msg)
	assert.ErrorIs(err, pubsub.ErrTimeout)
}
