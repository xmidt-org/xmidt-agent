// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/xmidt-org/eventor"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/event"
	"github.com/xmidt-org/xmidt-agent/internal/quic"
	"github.com/xmidt-org/xmidt-agent/internal/websocket"
)

var (
	ErrMisconfiguredCloud = errors.New("misconfigured Cloud")
	ErrQuicRequired       = errors.New("quic cannot be nil")
	ErrWsRequired         = errors.New("websocket cannot be nil")
)

const HandlerName = "proxy"

type CloudConfig struct {
	QuicPreferred bool
}

type Proxy struct {
	wg         sync.Mutex
	qc         *quic.QuicClient
	ws         *websocket.Websocket
	active     Handler
	preferQuic bool

	msgListeners eventor.Eventor[event.MsgListener]
	maxTries     int
}

// TODO - log handler stuff
func New(opts ...Option) (Handler, error) {
	p := &Proxy{}

	opts = append(opts,
		validateQuic(),
		validateWebsocket(),
	)

	for _, opt := range opts {
		if opt != nil {
			if err := opt.apply(p); err != nil {
				return nil, err
			}
		}
	}

	p.qc.AddConnectListener(
		event.ConnectListenerFunc(
			func(e event.Connect) {
				fmt.Printf("REMOVE %s connect event", p.qc.Name())
				if e.Err != nil && e.TriesSinceLastConnect > p.maxTries {
					fmt.Println("REMOVE switching to websocket")

					p.qc.Stop()

					fmt.Println("REMOVE after stop")

					p.ws.Start()

					p.wg.Lock()
					p.active = p.ws
					p.wg.Unlock()
				}
			}))

	p.ws.AddConnectListener(
		event.ConnectListenerFunc(
			func(e event.Connect) {
				fmt.Printf("REMOVE %s connect event", p.ws.Name())
				if e.Err != nil && e.TriesSinceLastConnect > p.maxTries {
					fmt.Println("REMOVE switching to quic")

					p.ws.Stop()
					p.qc.Start()

					p.wg.Lock()
					p.active = p.qc
					p.wg.Unlock()
				}
			}))

	p.AddProxyListeners(p.qc)
	p.AddProxyListeners(p.ws)

	if p.preferQuic {
		p.active = p.qc
	} else {
		p.active = p.ws
	}

	return p, nil
}

func (m *Proxy) Name() string {
	return m.active.Name()
}

func (m *Proxy) AddMessageListener(listener event.MsgListener) event.CancelFunc {
	return m.active.AddMessageListener(listener)
}

func (m *Proxy) Start() {
	m.active.Start()
}

func (m *Proxy) Stop() {
	m.active.Stop()
}

func (m *Proxy) Send(ctx context.Context, msg wrp.Message) error {
	return m.active.Send(ctx, msg)
}

func (p *Proxy) AddProxyListeners(handler Handler) {
	handler.AddMessageListener(
		event.MsgListenerFunc(
			func(m wrp.Message) {
				p.msgListeners.Visit(func(l event.MsgListener) {
					l.OnMessage(m)
				})
			}))

}

func (p *Proxy) HandleWrp(m wrp.Message) error {
	return p.active.Send(context.Background(), m)
}
