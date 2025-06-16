// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package cloud

import ()

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
			var nilHandler Handler
			if ws == nilHandler {
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
