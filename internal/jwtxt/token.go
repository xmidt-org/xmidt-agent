// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package jwtxt

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/xmidt-org/eventor"
	"github.com/xmidt-org/xmidt-agent/internal/jwtxt/event"
)

var (
	ErrInvalidConfig = errors.New("invalid configuration")
	ErrInvalidJWT    = errors.New("invalid jwt txt record")
	ErrInvalidPath   = errors.New("invalid path")
	ErrUnspportedID  = errors.New("unsupported device id")
	ErrUnspportedAlg = errors.New("unsupported jwt algorithm")
	ErrNoKeys        = errors.New("no keys provided")
	ErrNoKeysMatch   = errors.New("no keys match jwt")
	ErrInvalidInput  = errors.New("invalid input")
)

const (
	// DefaultTimeout is the default timeout for DNS queries.
	DefaultTimeout = time.Second * 15
)

// The Resolver interface allows users to provide their own resolver for
// resolving DNS TXT records.
type Resolver interface {
	LookupTXT(context.Context, string) ([]string, error)
}

type Instructions struct {
	// baseURL is the base url to examine for a JWT from a DNS TXT record.
	baseURL string

	// id is the identifier to prepend when looking for a DNS TXT record.
	id string

	// fqdn is the 'device_id.base_url' based on the input configuration.
	fqdn string

	// jwtOptions allows for normal and test configurations.
	jwtOptions []jwt.ParseOption

	// timeout is the timeout for the DNS query.
	timeout time.Duration

	// algorithms is the list of algorithms allowed for JWT validation.
	algorithms map[jwa.SignatureAlgorithm]struct{}

	// publicKeys is the collection of keys split out by supported algorithm.
	publicKeys []jwk.Key

	// The useable set of keys to use for validation.
	set jwk.Set

	// now is used to supply the current time that is needed for expiration.
	// it's here just for testing support.
	now func() time.Time

	// resolver is used to supply the resolver to use; it's here just for
	// testing support.
	resolver Resolver

	// fetchListeners calls back listeners when a fetch event occurs.
	fetchListeners eventor.Eventor[event.FetchListener]

	// ---- These fields are populated/used by the fetch method. ----

	// m protects the fields below.
	m sync.Mutex

	// validUntil is when the information from the JWT is valid until.
	validUntil time.Time

	// endpoint is the endpoint of the most recent valid JWT.
	endpoint string

	// payload is the payload of the most recent valid JWT.
	payload []byte
}

// New creates a new secure Instruction object.
func New(opts ...Option) (*Instructions, error) {
	ins := Instructions{
		now:        time.Now,
		resolver:   net.DefaultResolver,
		timeout:    DefaultTimeout,
		algorithms: map[jwa.SignatureAlgorithm]struct{}{},
	}

	full := append(opts,
		validateAlgs(),
		validateBase(),
		validateTheID(),
		makeSet(),
	)

	for _, opt := range full {
		if opt != nil {
			err := opt.apply(&ins)
			if err != nil {
				return nil, err
			}
		}
	}

	ins.fqdn = ins.id + "." + ins.baseURL

	return &ins, nil
}

func (ins *Instructions) dispatch(fe event.Fetch) error {
	ins.fetchListeners.Visit(func(listener event.FetchListener) {
		listener.OnFetchEvent(fe)
	})
	return fe.Err
}

// Endpoint returns the valid endpoint based on the instructions, or an error if
// there is no valid set of instructions.
func (ins *Instructions) Endpoint(ctx context.Context) (string, error) {
	ins.m.Lock()
	defer ins.m.Unlock()

	if ins.now().Before(ins.validUntil) {
		return ins.endpoint, nil
	}

	err := ins.fetch(ctx)
	if err != nil {
		return "", err
	}

	return ins.endpoint, nil
}

func (ins *Instructions) fetch(ctx context.Context) error {
	fe := event.Fetch{
		FQDN:            ins.fqdn,
		PriorExpiration: ins.validUntil,
	}

	// Don't wait forever if things are broken.
	ctx, cancel := context.WithTimeout(ctx, ins.timeout)
	defer cancel()

	fe.At = time.Now()
	lines, err := ins.resolver.LookupTXT(ctx, ins.fqdn)
	if err != nil {
		var dnsError *net.DNSError

		if errors.As(err, &dnsError) {
			fe.Server = dnsError.Server
			fe.Timeout = dnsError.Timeout()
			fe.TemporaryErr = dnsError.Temporary()
		} else {
			if ctx.Err() != nil {
				fe.Timeout = true
				fe.TemporaryErr = true
			}
		}
		fe.Duration = time.Since(fe.At)
		fe.Err = err
		return ins.dispatch(fe)
	}
	fe.Duration = time.Since(fe.At)

	fe.Found = true

	txt := ins.reassemble(lines)

	err = ins.validate(txt)
	if err != nil {
		fe.Err = err
		return ins.dispatch(fe)
	}

	fe.Endpoint = ins.endpoint
	fe.Expiration = ins.validUntil
	fe.Payload = ins.payload

	return ins.dispatch(fe)
}

// reassemble converts the TXT record from the list of encoded lines into
// the expected string of text that we all hope is a legit JWT.  The format
// of the lines in the TXT is:
//
//	00:base64_encoded_JWT_chuck_0
//	01:base64_encoded_JWT_chuck_1
//	nn:base64_encoded_JWT_chuck_nn
//
// Notes:
//   - the index could start at 0 or 1, so accept either.
//   - the lines get concatenated in order and all parts are needed
//   - only up to 100 lines are supported ... which is overkill for a JWT
//   - each line can be 255 bytes long including the leading 3 characters
//   - it doesn't really matter if we are missing something because the JWT
//     won't compute and will be discarded.
func (ins *Instructions) reassemble(lines []string) string {
	parts := make(map[string]string)

	// The value in the TXT record should be 1 (really 1, but make this tolerant
	// of 0 based indexing)
	parts["00"] = ""

	for _, line := range lines {
		segments := strings.Split(line, ":")
		if len(segments) != 2 {
			// skip empty or otherwise malformed lines.
			continue
		}
		parts[segments[0]] = segments[1]
	}

	getIndexString := func(i int) string { return fmt.Sprintf("%0.2d", i) }

	// Since we're re-assembling a JWT that is validated later, do the best
	// we can here, but don't be too strict.
	var buf strings.Builder

	for i := 0; i < len(parts); i++ {
		val, found := parts[getIndexString(i)]
		if !found {
			break
		}
		buf.WriteString(val)
	}

	return buf.String()
}

// validate takes a string that is believed to be a JWT and validates it.
// If it is valid, the information is saved in the Instruction object for
// use along with when the information is no longer valid after.
func (ins *Instructions) validate(input string) error {
	token, err := jwt.ParseString(input,
		jwt.WithKeySet(ins.set,
			jws.WithRequireKid(false),
			jws.WithInferAlgorithmFromKey(true),
		),
		jwt.WithClock(jwt.ClockFunc(ins.now)),
		jwt.WithRequiredClaim("endpoint"),
		jwt.WithValidate(true),
	)
	if err != nil {
		return errors.Join(err, ErrInvalidJWT)
	}

	msg, err := jws.ParseString(input)
	if err != nil {
		return errors.Join(err, ErrInvalidJWT)
	}

	ins.payload = msg.Payload()
	ins.validUntil = token.Expiration().Local()

	ep, _ := token.Get("endpoint")
	ins.endpoint = ep.(string)

	return nil
}
