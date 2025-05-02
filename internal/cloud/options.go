// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"github.com/xmidt-org/xmidt-agent/internal/event"
)

type Option interface {
	apply(*Proxy) error
}

type optionFunc func(*Proxy) error

func (f optionFunc) apply(p *Proxy) error {
	return f(p)
}

// whether or not to try quic before trying the websocket
func PreferQuic(preferQuic bool) Option {
	return optionFunc(
		func(p *Proxy) error {
			p.preferQuic = preferQuic
			return nil
		})
}

func QuicClient(qc Handler) Option {
	return optionFunc(
		func(p *Proxy) error {
			if qc == nil {
				return ErrQuicRequired
			}
			p.qc = qc
			return nil
		})
}

func Websocket(ws Handler) Option {
	return optionFunc(
		func(p *Proxy) error {
			if ws == nil {
				return ErrWsRequired
			}
			p.ws = ws
			return nil
		})
}

// max tries before switching protocols (quic vs websocket)
func MaxTries(maxTries int32) Option {
	return optionFunc(
		func(p *Proxy) error {
			p.maxTries = maxTries
			return nil
		})
}

// AddMessageListener adds a message listener to the WS connection.
// The listener will be called for every message received from the WS.
func AddMessageListener(listener event.MsgListener, cancel ...*event.CancelFunc) Option {
	return optionFunc(
		func(p *Proxy) error {
			var ignored event.CancelFunc
			cancel = append(cancel, &ignored)
			*cancel[0] = event.CancelFunc(p.msgListeners.Add(listener))
			return nil
		})
}
