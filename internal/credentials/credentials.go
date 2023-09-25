// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xmidt-org/eventor"
	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/credentials/event"
)

var (
	ErrInvalidInput      = fmt.Errorf("invalid input")
	ErrNilRequest        = fmt.Errorf("nil request")
	ErrNoToken           = fmt.Errorf("no token")
	ErrTokenExpired      = fmt.Errorf("token expired")
	ErrFetchNotAttempted = fmt.Errorf("fetch not attempted")
	ErrFetchFailed       = fmt.Errorf("fetch failed")
)

const (
	DefaultRefetchPercent = 90.0
)

/*
Notes:
  - The network interface is set via the http.Client.
  - If v4, v6 or both are desired, it is set via the http.Client.
  - The timeout is set via the http.Client.
  - The maximum redirect count is set via the http.Client.
  - mTLS is set via the http.Client.
  - The TLS version is set via the http.Client.
*/
type Credentials struct {
	m                 sync.RWMutex
	wg                sync.WaitGroup
	shutdown          context.CancelFunc
	fetched           chan struct{}
	valid             chan struct{}
	wakeup            chan chan struct{}
	nowFunc           func() time.Time
	fetchListeners    eventor.Eventor[event.FetchListener]
	decorateListeners eventor.Eventor[event.DecorateListener]

	// What we are using to fetch the credentials.

	url                  string
	refetchPercent       float64
	assumedLifetime      time.Duration
	client               *http.Client
	macAddress           wrp.DeviceID
	serialNumber         string
	hardwareModel        string
	hardwareManufacturer string
	firmwareVersion      string
	lastRebootReason     string
	xmidtProtocol        string
	bootRetryWait        time.Duration
	lastReconnectReason  func() string // dynamic
	partnerID            func() string // dynamic

	// What we are using to decorate the request.
	token *xmidtToken
}

// Option is the interface implemented by types that can be used to
// configure the credentials.
type Option interface {
	apply(*Credentials) error
}

// New creates a new credentials service object.
func New(opts ...Option) (*Credentials, error) {
	required := []Option{
		urlVador(),
		macAddressVador(),
		serialNumberVador(),
		hardwareModelVador(),
		hardwareManufacturerVador(),
		firmwareVersionVador(),
		lastRebootReasonVador(),
		xmidtProtocolVador(),
		bootRetryWaitVador(),
	}

	c := Credentials{
		client:              http.DefaultClient,
		fetched:             make(chan struct{}),
		valid:               make(chan struct{}),
		wakeup:              make(chan chan struct{}),
		nowFunc:             time.Now,
		refetchPercent:      DefaultRefetchPercent,
		lastReconnectReason: func() string { return "" },
		partnerID:           func() string { return "" },
	}

	opts = append(opts, required...)

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		err := opt.apply(&c)
		if err != nil {
			return nil, err
		}
	}

	return &c, nil
}

// Start starts the credentials service.
func (c *Credentials) Start() {
	c.m.Lock()
	defer c.m.Unlock()

	if c.shutdown != nil {
		return
	}

	var ctx context.Context
	ctx, c.shutdown = context.WithCancel(context.Background())

	go c.run(ctx)
}

// Stop stops the credentials service.
func (c *Credentials) Stop() {
	c.m.Lock()
	shudown := c.shutdown
	c.m.Unlock()

	if shudown != nil {
		shudown()
	}
	c.wg.Wait()
}

// WaitUntilFetched blocks until an attempt to fetch the credentials has been
// made or the context is canceled.
func (c *Credentials) WaitUntilFetched(ctx context.Context) {
	// Fetched is never re-created, so we don't need to lock.
	select {
	case <-c.fetched:
	case <-ctx.Done():
	}
}

// WaitUntilValid blocks until the credentials are valid or the context is
// canceled.
func (c *Credentials) WaitUntilValid(ctx context.Context) {
	c.m.RLock()
	valid := c.valid
	c.m.RUnlock()

	select {
	case <-valid:
	case <-ctx.Done():
	}
}

// MarkInvalid marks the credentials as invalid and causes the service to
// immediately attempt to fetch new credentials.
func (c *Credentials) MarkInvalid(ctx context.Context) {
	ch := make(chan struct{})

	select {
	case c.wakeup <- ch:
		select {
		case <-ch:
		case <-ctx.Done():
		}
	case <-ctx.Done():
	}

}

// Decorate decorates the request with the credentials.  If the credentials
// are not valid, an error is returned.
func (c *Credentials) Decorate(req *http.Request) error {
	var e event.Decorate

	if req == nil {
		e.Err = ErrNilRequest
		return c.dispatch(e)
	}

	var token string
	var expiresAt time.Time

	c.m.RLock()
	if c.token != nil {
		token = c.token.Token
		expiresAt = c.token.ExpiresAt
	}
	c.m.RUnlock()

	if token == "" {
		e.Err = ErrNoToken
		return c.dispatch(e)
	}

	e.Expiration = expiresAt
	if c.nowFunc().After(expiresAt) {
		e.Err = ErrTokenExpired
		return c.dispatch(e)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	return c.dispatch(e)
}

// fetch fetches the credentials from the server.  This should only be called
// by the run() method.
func (c *Credentials) fetch(ctx context.Context) (*xmidtToken, time.Duration, error) {
	var fe event.Fetch

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		fe.Err = errors.Join(err, ErrFetchNotAttempted)
		return nil, 0, c.dispatch(fe)
	}

	tid, err := uuid.NewRandom()
	if err != nil {
		fe.Err = errors.Join(err, ErrFetchNotAttempted)
		return nil, 0, c.dispatch(fe)
	}

	fe.UUID = tid

	req.Header.Set("X-Midt-Boot-Retry-Wait", c.bootRetryWait.String())
	req.Header.Set("X-Midt-Mac-Address", c.macAddress.ID())
	req.Header.Set("X-Midt-Serial-Number", c.serialNumber)
	req.Header.Set("X-Midt-Uuid", tid.String())
	req.Header.Set("X-Midt-Partner-Id", c.partnerID())
	req.Header.Set("X-Midt-Hardware-Model", c.hardwareModel)
	req.Header.Set("X-Midt-Hardware-Manufacturer", c.hardwareManufacturer)
	req.Header.Set("X-Midt-Firmware-Name", c.firmwareVersion)
	req.Header.Set("X-Midt-Protocol", c.xmidtProtocol)
	req.Header.Set("X-Midt-Last-Reboot-Reason", c.lastRebootReason)
	req.Header.Set("X-Midt-Last-Reconnect-Reason", c.lastReconnectReason())

	fe.At = time.Now()
	resp, err := c.client.Do(req)
	fe.Duration = time.Since(fe.At)
	if err != nil {
		fe.Err = errors.Join(err, ErrFetchFailed)
		return nil, 0, c.dispatch(fe)
	}
	defer resp.Body.Close()

	fe.StatusCode = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		var retryIn time.Duration
		if resp.StatusCode == http.StatusTooManyRequests {
			if after, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil {
				retryIn = time.Duration(after) * time.Second
			}
		}

		fe.RetryIn = retryIn
		fe.Err = errors.Join(err, ErrFetchFailed)
		return nil, retryIn, c.dispatch(fe)
	}

	var token xmidtToken
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fe.Err = errors.Join(err, ErrFetchFailed)
		return nil, 0, c.dispatch(fe)
	}
	token.Token = string(body)

	// One hundred years is forever.
	token.ExpiresAt = c.nowFunc().Add(time.Hour * 24 * 365 * 100)
	if c.assumedLifetime > 0 {
		// If we have an assumed lifetime, use it.
		token.ExpiresAt = c.nowFunc().Add(c.assumedLifetime)
	}

	if expiration, err := http.ParseTime(resp.Header.Get("Expires")); err == nil {
		// Even better, we were told when it expires.
		token.ExpiresAt = expiration
	}

	fe.Expiration = token.ExpiresAt

	return &token, 0, c.dispatch(fe)
}

// run is the main loop for the credentials service.
func (c *Credentials) run(ctx context.Context) {
	var (
		timer   *time.Timer
		fetched bool
		valid   bool
	)

	c.wg.Add(1)
	defer c.wg.Done()

	for {
		token, retryIn, err := c.fetch(ctx)
		if !fetched {
			close(c.fetched)
			fetched = true
		}

		// Assume we failed, so retry in 1 second or when the server suggested.
		next := max(time.Second, retryIn)

		if err == nil && token != nil {
			expires := token.ExpiresAt

			c.m.Lock()
			c.token = token
			c.m.Unlock()

			if !valid {
				close(c.valid)
				valid = true
			}

			until := expires.Sub(c.nowFunc())
			if 0 < until {
				// Add a timer to fetch the token again
				next = time.Duration(float64(until) * c.refetchPercent / 100.0)
			}
		}

		timer = time.NewTimer(next)
		defer timer.Stop()

		select {
		case ch := <-c.wakeup:
			if valid {
				c.m.Lock()
				c.valid = make(chan struct{})
				valid = false
				c.m.Unlock()
			}
			ch <- struct{}{}
		case <-timer.C:
		case <-ctx.Done():
			return
		}
	}
}

// dispatch dispatches the event to the listeners and returns the error that
// should be returned by the caller.
func (c *Credentials) dispatch(evnt any) error {
	switch evnt := evnt.(type) {
	case event.Fetch:
		c.fetchListeners.Visit(func(listener event.FetchListener) {
			listener.OnFetch(evnt)
		})
		return evnt.Err
	case event.Decorate:
		c.decorateListeners.Visit(func(listener event.DecorateListener) {
			listener.OnDecorate(evnt)
		})
		return evnt.Err
	}

	panic("unknown event type")
}

// xmidtToken is the token returned from the server as well as the expiration
// time.
type xmidtToken struct {
	Token     string
	ExpiresAt time.Time
}
