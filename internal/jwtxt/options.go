// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package jwtxt

import (
	"fmt"
	"net/url"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/xmidt-org/wrp-go/v4"
	"github.com/xmidt-org/xmidt-agent/internal/jwtxt/event"
)

// Option is a functional option for the Instructions constructor.
type Option interface {
	apply(*Instructions) error
}

func errorOptionFn(err error) Option {
	return errorOption{
		err: err,
	}
}

type errorOption struct {
	err error
}

func (e errorOption) apply(*Instructions) error {
	return e.err
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

var allowedSigningAlgorithms = map[string]jwa.SignatureAlgorithm{
	"EdDSA": jwa.EdDSA,
	"ES256": jwa.ES256,
	"ES384": jwa.ES384,
	"ES512": jwa.ES512,
	"PS256": jwa.PS256,
	"PS384": jwa.PS384,
	"PS512": jwa.PS512,
	"RS256": jwa.RS256,
	"RS384": jwa.RS384,
	"RS512": jwa.RS512,
}

// Algorithms sets the algorithms to use for verification.  Valid algorithms
// are "EdDSA", "ES256", "ES384", "ES512", "PS256", "PS384", "PS512",
// "RS256", "RS384", and "RS512".
func Algorithms(algs ...string) Option {
	allowed := make([]jwa.SignatureAlgorithm, 0, len(algs))

	for _, alg := range algs {
		got, found := allowedSigningAlgorithms[alg]
		if !found {
			return errorOptionFn(fmt.Errorf("%w '%s'", ErrUnspportedAlg, alg))
		}
		allowed = append(allowed, got)
	}

	return &algorithms{
		algs: allowed,
	}
}

type algorithms struct {
	algs []jwa.SignatureAlgorithm
}

func (a algorithms) apply(ins *Instructions) error {
	for _, alg := range a.algs {
		ins.algorithms[alg] = struct{}{}
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
	for _, single := range p.pems {
		key, err := jwk.ParseKey(single, jwk.WithPEM(true))
		if err != nil {
			return fmt.Errorf("%w: invalid pem", ErrInvalidInput)
		}

		algs, err := jws.AlgorithmsForKey(key)
		if err != nil {
			return fmt.Errorf("%w: invalid key", ErrInvalidInput)
		}

		for _, a := range algs {
			if _, ok := allowedSigningAlgorithms[a.String()]; !ok {
				return fmt.Errorf("%w: algorithm not allowed %s", ErrInvalidInput, a)
			}
		}

		ins.publicKeys = append(ins.publicKeys, key)
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

// -- validation options -------------------------------------------------------

func validateAlgs() Option {
	return &validateAlgorithms{}
}

type validateAlgorithms struct{}

func (validateAlgorithms) apply(ins *Instructions) error {
	if len(ins.algorithms) == 0 {
		return fmt.Errorf("%w: zero provided algorithms", ErrInvalidInput)
	}

	if len(ins.publicKeys) == 0 {
		return fmt.Errorf("%w: zero provided public keys", ErrInvalidInput)
	}

	// Ensure each key passed in provides an allowed algorithm.
	for _, k := range ins.publicKeys {
		var allowed bool

		algs, _ := jws.AlgorithmsForKey(k)
		for _, alg := range algs {
			if _, ok := ins.algorithms[alg]; ok {
				allowed = true
				break
			}
		}

		if !allowed {
			return fmt.Errorf("%w: provided pem does not support allowed algorithm", ErrInvalidInput)
		}
	}

	return nil
}

func validateBase() Option {
	return &validateBaseURL{}
}

type validateBaseURL struct{}

func (validateBaseURL) apply(ins *Instructions) error {
	if ins.baseURL == "" {
		return fmt.Errorf("%w: baseURL must be set", ErrInvalidInput)
	}

	return nil
}

func validateTheID() Option {
	return &validateID{}
}

type validateID struct{}

func (validateID) apply(ins *Instructions) error {
	if ins.id == "" {
		return fmt.Errorf("%w: id must be set", ErrInvalidInput)
	}

	return nil
}

// -- internal options ---------------------------------------------------------

func makeSet() Option {
	return &makeSetOption{}
}

type makeSetOption struct{}

func (makeSetOption) apply(ins *Instructions) error {
	ins.set = jwk.NewSet()

	for _, k := range ins.publicKeys {
		ins.set.AddKey(k)
	}
	return nil
}
