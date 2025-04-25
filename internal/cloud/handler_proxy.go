// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"errors"
	"fmt"
	"sync"

	"github.com/xmidt-org/eventor"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/event"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
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
	wg sync.Mutex

	qc Handler
	ws Handler

	active           Handler
	activeWrpHandler wrpkit.Handler

	preferQuic bool

	msgListeners     eventor.Eventor[event.MsgListener]
	connectListeners eventor.Eventor[event.ConnectListener]
	maxTries         int
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
			p.OnQuicConnect,
		))

	p.ws.AddConnectListener(
		event.ConnectListenerFunc(
			p.OnWebsocketConnect,
		))

	p.AddProxyListeners(p.qc)
	p.AddProxyListeners(p.ws)

	if p.preferQuic {
		p.active = p.qc
		p.activeWrpHandler = p.qc.(wrpkit.Handler)
	} else {
		p.active = p.ws
		p.activeWrpHandler = p.ws.(wrpkit.Handler)
	}

	return p, nil
}

func (p *Proxy) Name() string {
	return p.active.Name()
}

func (p *Proxy) AddMessageListener(listener event.MsgListener) event.CancelFunc {
	return event.CancelFunc(p.msgListeners.Add(listener))
}

func (p *Proxy) AddConnectListener(listener event.ConnectListener) event.CancelFunc {
	return event.CancelFunc(p.connectListeners.Add(listener))
}

func (m *Proxy) Start() {
	m.active.Start()
}

func (m *Proxy) Stop() {
	m.active.Stop()
}

// func (m *Proxy) Send(ctx context.Context, msg wrp.Message) error {
// 	// no op
// 	return nil
// }

func (p *Proxy) AddProxyListeners(handler Handler) {
	handler.AddMessageListener(
		event.MsgListenerFunc(
			func(m wrp.Message) {
				p.msgListeners.Visit(func(l event.MsgListener) {
					l.OnMessage(m)
				})
			}))

	handler.AddConnectListener(
		event.ConnectListenerFunc(
			func(e event.Connect) {
				p.connectListeners.Visit(func(l event.ConnectListener) {
					l.OnConnect(e)
				})
			}))

}

func (p *Proxy) HandleWrp(m wrp.Message) error {
	return p.activeWrpHandler.HandleWrp(m)
}

func (p *Proxy) OnQuicConnect(e event.Connect) {
	fmt.Printf("REMOVE %s connect event", p.qc.Name())
	if e.Err != nil && e.TriesSinceLastConnect > p.maxTries {
		fmt.Println("REMOVE switching to websocket")

		p.qc.Stop()

		fmt.Println("REMOVE after stop")

		p.ws.Start()

		p.wg.Lock()
		p.active = p.ws
		p.activeWrpHandler = p.ws.(wrpkit.Handler)
		p.wg.Unlock()
	}
}

func (p *Proxy) OnWebsocketConnect(e event.Connect) {
	fmt.Printf("REMOVE %s connect event", p.ws.Name())
	if e.Err != nil && e.TriesSinceLastConnect > p.maxTries {
		fmt.Println("REMOVE switching to quic")

		p.ws.Stop()
		p.qc.Start()

		p.wg.Lock()

		p.active = p.qc
		p.activeWrpHandler = p.qc.(wrpkit.Handler)

		p.wg.Unlock()
	}
}
