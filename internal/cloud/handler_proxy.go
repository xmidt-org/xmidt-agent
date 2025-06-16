// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"errors"
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

type CloudConfig struct {
	QuicPreferred bool
}

type Proxy struct {
	wg sync.Mutex

	qc Handler
	ws Handler

	qcMsgHandler wrpkit.Handler
	wsMsgHandler wrpkit.Handler

	active           Handler
	activeWrpHandler wrpkit.Handler

	preferQuic bool

	msgListeners     eventor.Eventor[event.MsgListener]
	connectListeners eventor.Eventor[event.ConnectListener]
	maxTries         int32
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

	p.qcMsgHandler = p.qc.(wrpkit.Handler)
	p.wsMsgHandler = p.ws.(wrpkit.Handler)

	p.wg.Lock()
	defer p.wg.Unlock()
	if p.preferQuic {
		p.active = p.qc
		p.activeWrpHandler = p.qcMsgHandler
	} else {
		p.active = p.ws
		p.activeWrpHandler = p.wsMsgHandler
	}

	return p, nil
}

func (p *Proxy) Name() string {
	p.wg.Lock()
	defer p.wg.Unlock()
	return p.active.Name()
}

func (p *Proxy) AddMessageListener(listener event.MsgListener) event.CancelFunc {
	return event.CancelFunc(p.msgListeners.Add(listener))
}

func (p *Proxy) AddConnectListener(listener event.ConnectListener) event.CancelFunc {
	return event.CancelFunc(p.connectListeners.Add(listener))
}

func (p *Proxy) Start() {
	p.wg.Lock()
	defer p.wg.Unlock()
	p.active.Start()
}

func (p *Proxy) Stop() {
	p.wg.Lock()
	defer p.wg.Unlock()
	p.active.Stop()
}

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
	p.wg.Lock()
	defer p.wg.Unlock()
	return p.activeWrpHandler.HandleWrp(m)
}

func (p *Proxy) OnQuicConnect(e event.Connect) {

	// should we switch to websocket?
	if e.Err != nil && e.TriesSinceLastConnect > p.maxTries {
		p.qc.Stop()
		p.ws.Start()

		p.wg.Lock()
		defer p.wg.Unlock()
		p.active = p.ws
		p.activeWrpHandler = p.wsMsgHandler
	}
}

func (p *Proxy) OnWebsocketConnect(e event.Connect) {
	// should we switch to quic?
	if e.Err != nil && e.TriesSinceLastConnect > p.maxTries {
		p.ws.Stop()
		p.qc.Start()

		p.wg.Lock()
		defer p.wg.Unlock()
		p.active = p.qc
		p.activeWrpHandler = p.qcMsgHandler
	}
}
