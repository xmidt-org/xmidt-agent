// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package jwtxt

import (
	"fmt"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/jwtxt/event"
)

// Option is a functional option for the Instructions constructor.
type Option interface {
	apply(*Instructions) error
}

// WithFetchListener adds a listener for fetch events.
func WithFetchListener(listener event.FetchListener) Option {
	return &fetchListener{
		listener: listener,
	}
}

type fetchListener struct {
	listener event.FetchListener
}

func (f fetchListener) apply(ins *Instructions) error {
	ins.fetchListeners.Add(f.listener)
	return nil
}

// UseResolver sets the resolver to use for DNS queries.
func UseResolver(resolver Resolver) Option {
	return &useResolver{
		resolver: resolver,
	}
}

type useResolver struct {
	resolver Resolver
}

func (u useResolver) apply(ins *Instructions) error {
	ins.resolver = u.resolver
	return nil
}

// UseNowFunc sets the function to use for getting the current time.
func UseNowFunc(nowFunc func() time.Time) Option {
	return &useNowFunc{
		now: nowFunc,
	}
}

type useNowFunc struct {
	now func() time.Time
}

func (u useNowFunc) apply(ins *Instructions) error {
	ins.now = u.now
	return nil
}

// Algorithms sets the algorithms to use for verification.  Valid algorithms
// are "EdDSA", "ES256", "ES384", "ES512", "PS256", "PS384", "PS512",
// "RS256", "RS384", and "RS512".
func Algorithms(algs ...string) Option {
	return &algorithms{
		algs: algs,
	}
}

type algorithms struct {
	algs []string
}

func (a algorithms) apply(ins *Instructions) error {
	for _, alg := range a.algs {
		switch alg {
		case "EdDSA",
			"ES256", "ES384", "ES512",
			"PS256", "PS384", "PS512",
			"RS256", "RS384", "RS512":
		default:
			return fmt.Errorf("%w '%s'", ErrUnspportedAlg, alg)
		}
		ins.jwtOptions = append(ins.jwtOptions, jwt.WithValidMethods([]string{alg}))
		ins.algorithms = append(ins.algorithms, alg)
	}
	return nil
}

// Timeout sets the timeout for DNS queries.  0 means use the default timeout.
// A negative timeout is invalid.
func Timeout(timeout time.Duration) Option {
	return &timeoutOption{
		timeout: timeout,
	}
}

type timeoutOption struct {
	timeout time.Duration
}

func (t timeoutOption) apply(ins *Instructions) error {
	if t.timeout < 0 {
		return fmt.Errorf("%w: timeout is invalid %s", ErrInvalidInput, t.timeout)
	}
	if t.timeout == 0 {
		t.timeout = DefaultTimeout
	}
	ins.timeout = t.timeout
	return nil
}

// WithPEMs adds PEM-encoded keys to the list of keys to use for verification.
func WithPEMs(pems ...[]byte) Option {
	return &pemOption{
		pems: pems,
	}
}

type pemOption struct {
	pems [][]byte
}

func (p pemOption) apply(ins *Instructions) error {
	for _, pem := range p.pems {
		var key jwt.VerificationKey
		var err error

		key, err = jwt.ParseECPublicKeyFromPEM(pem)

		if err != nil {
			key, err = jwt.ParseRSAPublicKeyFromPEM(pem)
		}

		if err != nil {
			key, err = jwt.ParseEdPublicKeyFromPEM(pem)
		}

		if err != nil {
			return fmt.Errorf("%w: invalid pem", ErrInvalidInput)
		}

		ins.publicKeys.Keys = append(ins.publicKeys.Keys, key)
	}

	return nil
}

// BaseURL sets the base URL to use for the endpoint.
func BaseURL(url string) Option {
	return &baseURL{
		url: url,
	}
}

type baseURL struct {
	url string
}

func (b baseURL) apply(ins *Instructions) error {
	u, err := url.ParseRequestURI(b.url)
	if err != nil {
		return fmt.Errorf("%w: invalid url %s", ErrInvalidInput, b.url)
	}

	ins.baseURL = u.Hostname()
	return nil
}

// DeviceID sets the ID to use for the endpoint.
func DeviceID(id string) Option {
	return &idOption{
		id: id,
	}
}

type idOption struct {
	id string
}

func (i idOption) apply(ins *Instructions) error {
	id, err := wrp.ParseDeviceID(i.id)
	if err != nil {
		return fmt.Errorf("%w: invalid id %s", ErrInvalidInput, i.id)
	}
	ins.id = id.ID()
	return nil
}
