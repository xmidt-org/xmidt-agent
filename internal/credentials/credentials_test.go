// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/credentials/event"
)

func TestNew(t *testing.T) {
	testClient := &http.Client{}

	simplest := []Option{
		URL("http://example.com"),
		MacAddress(wrp.DeviceID("mac:112233445566")),
		SerialNumber("1234567890"),
		HardwareModel("model"),
		HardwareManufacturer("manufacturer"),
		FirmwareVersion("version"),
		LastRebootReason("reason"),
		XmidtProtocol("protocol"),
		BootRetryWait(1),
	}

	tests := []struct {
		description string
		opt         Option
		opts        []Option
		expectedErr error
		check       func(*assert.Assertions, *Credentials)
		checks      []func(*assert.Assertions, *Credentials)
		optStr      string
	}{
		{
			description: "nil option",
			expectedErr: ErrInvalidInput,
		}, {
			description: "simplest config",
			opts:        simplest,
			check: func(assert *assert.Assertions, c *Credentials) {
				assert.Equal("http://example.com", c.url)
				assert.Equal(wrp.DeviceID("mac:112233445566"), c.macAddress)
				assert.Equal("1234567890", c.serialNumber)
				assert.Equal("model", c.hardwareModel)
				assert.Equal("manufacturer", c.hardwareManufacturer)
				assert.Equal("version", c.firmwareVersion)
				assert.Equal("reason", c.lastRebootReason)
				assert.Equal("protocol", c.xmidtProtocol)
				assert.Equal(time.Duration(1), c.bootRetryWait)
				assert.Empty(c.partnerID())
				assert.Empty(c.lastReconnectReason())
			},
		}, {
			description: "common config",
			opts: append(simplest, []Option{
				HTTPClient(testClient),
				RefetchPercent(50.0),
				PartnerID(func() string { return "partner" }),
				AssumedLifetime(24 * time.Hour),
				LastReconnectReason(func() string { return "reconnect_reason" }),
			}...),
			check: func(assert *assert.Assertions, c *Credentials) {
				assert.Equal("http://example.com", c.url)
				assert.Equal(wrp.DeviceID("mac:112233445566"), c.macAddress)
				assert.Equal("1234567890", c.serialNumber)
				assert.Equal("model", c.hardwareModel)
				assert.Equal("manufacturer", c.hardwareManufacturer)
				assert.Equal("version", c.firmwareVersion)
				assert.Equal("reason", c.lastRebootReason)
				assert.Equal("protocol", c.xmidtProtocol)
				assert.Equal(time.Duration(1), c.bootRetryWait)
				assert.Equal(testClient, c.client)
				assert.Equal(50.0, c.refetchPercent)
				assert.Equal("partner", c.partnerID())
				assert.Equal(24*time.Hour, c.assumedLifetime)
				assert.Equal("reconnect_reason", c.lastReconnectReason())
			},
		}, {
			description: "invalid url",
			opts: append(simplest, []Option{
				URL(""),
			}...),
			expectedErr: ErrInvalidInput,
		}, {
			description: "invalid mac address",
			opts: append(simplest, []Option{
				MacAddress(wrp.DeviceID("")),
			}...),
			expectedErr: ErrInvalidInput,
		}, {
			description: "invalid serial number",
			opts: append(simplest, []Option{
				SerialNumber(""),
			}...),
			expectedErr: ErrInvalidInput,
		}, {
			description: "invalid hardware model",
			opts: append(simplest, []Option{
				HardwareModel(""),
			}...),
			expectedErr: ErrInvalidInput,
		}, {
			description: "invalid hardware manufacturer",

			opts: append(simplest, []Option{
				HardwareManufacturer(""),
			}...),
			expectedErr: ErrInvalidInput,
		}, {
			description: "invalid firmware version",
			opts: append(simplest, []Option{
				FirmwareVersion(""),
			}...),
			expectedErr: ErrInvalidInput,
		}, {
			description: "invalid last reboot reason",
			opts: append(simplest, []Option{
				LastRebootReason(""),
			}...),
			expectedErr: ErrInvalidInput,
		}, {
			description: "invalid xmidt protocol",
			opts: append(simplest, []Option{
				XmidtProtocol(""),
			}...),
			expectedErr: ErrInvalidInput,
		}, {
			description: "invalid boot retry wait",
			opts: append(simplest, []Option{
				BootRetryWait(0),
			}...),
			expectedErr: ErrInvalidInput,
		}, {
			description: "refetch percent (default)",
			opts: append(simplest, []Option{
				RefetchPercent(0.0),
			}...),
			check: func(assert *assert.Assertions, c *Credentials) {
				assert.Equal(DefaultRefetchPercent, c.refetchPercent)
			},
		}, {
			description: "invalid refetch percent (low)",
			opts: append(simplest, []Option{
				RefetchPercent(-1.0),
			}...),
			expectedErr: ErrInvalidInput,
		}, {
			description: "invalid refetch percent (high)",
			opts: append(simplest, []Option{
				RefetchPercent(100.1),
			}...),
			expectedErr: ErrInvalidInput,
		}, {
			description: "invalid http client",
			opts: append(simplest, []Option{
				HTTPClient(nil),
			}...),
			check: func(assert *assert.Assertions, c *Credentials) {
				assert.NotNil(c.client)
			},
		}, {
			description: "invalid partner id",
			opts: append(simplest, []Option{
				PartnerID(nil),
			}...),
			check: func(assert *assert.Assertions, c *Credentials) {
				assert.NotNil(c.partnerID)
			},
		}, {
			description: "invalid last reconnect reason",
			opts: append(simplest, []Option{
				LastReconnectReason(nil),
			}...),
			check: func(assert *assert.Assertions, c *Credentials) {
				assert.NotNil(c.lastReconnectReason)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			opts := append(tc.opts, tc.opt)

			got, err := New(opts...)

			checks := append(tc.checks, tc.check)
			for _, check := range checks {
				if check != nil {
					check(assert, got)
				}
			}

			if tc.expectedErr == nil {
				assert.NotNil(got)
				assert.NoError(err)
				return
			}

			assert.Nil(got)
			assert.ErrorIs(err, tc.expectedErr)
		})
	}
}

func TestEndToEnd429(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				r.Body.Close()

				w.Header().Add("Retry-After", "15")
				w.WriteHeader(http.StatusTooManyRequests)
			},
		),
	)
	defer server.Close()

	var called int
	c, err := New(
		URL(server.URL),
		MacAddress(wrp.DeviceID("mac:112233445566")),
		SerialNumber("1234567890"),
		HardwareModel("model"),
		HardwareManufacturer("manufacturer"),
		FirmwareVersion("version"),
		LastRebootReason("reason"),
		XmidtProtocol("protocol"),
		BootRetryWait(1),
		AddFetchListener(event.FetchListenerFunc(
			func(e event.Fetch) {
				assert.Equal(15*time.Second, e.RetryIn)
				assert.ErrorIs(e.Err, ErrFetchFailed)
				called++
			})),
	)

	require.NoError(err)
	require.NotNil(c)

	c.Start()
	defer c.Stop()

	ctx := context.Background()
	deadline, cancel := context.WithDeadline(ctx, time.Now().Add(1*time.Second))
	defer cancel()
	c.WaitUntilFetched(deadline)
	assert.Equal(1, called)
}

func TestEndToEndWithExpires(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	when := time.Now().Add(1 * time.Hour)

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				r.Body.Close()

				w.Header().Add("Expires", when.Format(http.TimeFormat))
				_, _ = w.Write([]byte(`token`))
			},
		),
	)
	defer server.Close()

	c, err := New(
		URL(server.URL),
		MacAddress(wrp.DeviceID("mac:112233445566")),
		SerialNumber("1234567890"),
		HardwareModel("model"),
		HardwareManufacturer("manufacturer"),
		FirmwareVersion("version"),
		LastRebootReason("reason"),
		XmidtProtocol("protocol"),
		BootRetryWait(1),
		AddFetchListener(event.FetchListenerFunc(
			func(e event.Fetch) {
				assert.Equal(when.Format(http.TimeFormat), e.Expiration.Format(http.TimeFormat))
				assert.NoError(e.Err)
			})),
	)

	require.NoError(err)
	require.NotNil(c)

	c.Start()
	defer c.Stop()

	ctx := context.Background()
	deadline, cancel := context.WithDeadline(ctx, time.Now().Add(1*time.Second))
	defer cancel()
	c.WaitUntilValid(deadline)
}

func TestEndToEndMarkInvalid(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	counter := 1

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				r.Body.Close()

				_, _ = w.Write([]byte(`token`))
				counter++
			},
		),
	)
	defer server.Close()

	called := 0
	c, err := New(
		URL(server.URL),
		MacAddress(wrp.DeviceID("mac:112233445566")),
		SerialNumber("1234567890"),
		HardwareModel("model"),
		HardwareManufacturer("manufacturer"),
		FirmwareVersion("version"),
		LastRebootReason("reason"),
		XmidtProtocol("protocol"),
		BootRetryWait(1),
		AddFetchListener(event.FetchListenerFunc(
			func(e event.Fetch) {
				fmt.Println("Fetch:")
				fmt.Printf("  At:         %s\n", e.At.Format(time.RFC3339Nano))
				fmt.Printf("  Duration:   %s\n", e.Duration)
				fmt.Printf("  UUID:       %s\n", e.UUID)
				fmt.Printf("  StatusCode: %d\n", e.StatusCode)
				fmt.Printf("  RetryIn:    %s\n", e.RetryIn)
				fmt.Printf("  Expiration: %s\n", e.Expiration.Format(time.RFC3339))
				assert.NoError(e.Err)
				called++
			})),
	)

	require.NoError(err)
	require.NotNil(c)

	c.Start()
	defer c.Stop()

	ctx := context.Background()
	deadline, cancel := context.WithDeadline(ctx, time.Now().Add(1*time.Second))
	defer cancel()
	c.WaitUntilValid(deadline)

	c.MarkInvalid(deadline)

	c.WaitUntilValid(deadline)

	assert.Equal(2, called)
}

func TestEndToEnd(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				r.Body.Close()

				_, _ = w.Write([]byte(`token`))
			},
		),
	)
	defer server.Close()

	c, err := New(
		URL(server.URL),
		MacAddress(wrp.DeviceID("mac:112233445566")),
		SerialNumber("1234567890"),
		HardwareModel("model"),
		HardwareManufacturer("manufacturer"),
		FirmwareVersion("version"),
		LastRebootReason("reason"),
		XmidtProtocol("protocol"),
		BootRetryWait(1),
		AssumedLifetime(24*time.Hour),
		AddFetchListener(event.FetchListenerFunc(
			func(e event.Fetch) {
				fmt.Println("Fetch:")
				fmt.Printf("  At:         %s\n", e.At.Format(time.RFC3339))
				fmt.Printf("  Duration:   %s\n", e.Duration)
				fmt.Printf("  UUID:       %s\n", e.UUID)
				fmt.Printf("  StatusCode: %d\n", e.StatusCode)
				fmt.Printf("  RetryIn:    %s\n", e.RetryIn)
				fmt.Printf("  Expiration: %s\n", e.Expiration.Format(time.RFC3339))
				if e.Err != nil {
					fmt.Printf("  Err:        %s\n", e.Err)
				} else {
					fmt.Println("  Err:        nil")
				}
			})),
		AddDecorateListener(event.DecorateListenerFunc(
			func(e event.Decorate) {
				fmt.Println("Decorate:")
				fmt.Printf("  Expiration: %s\n", e.Expiration.Format(time.RFC3339))
				if e.Err != nil {
					fmt.Printf("  Err:        %s\n", e.Err)
				} else {
					fmt.Println("  Err:        nil")
				}
			})),
	)

	require.NoError(err)
	require.NotNil(c)

	c.Start()

	// Multiple calls to Start is ok.
	c.Start()

	ctx := context.Background()
	deadline, cancel := context.WithDeadline(ctx, time.Now().Add(1*time.Second))
	c.WaitUntilFetched(deadline)
	c.WaitUntilValid(deadline)
	cancel()

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	assert.NoError(err)
	assert.NotNil(req)

	err = c.Decorate(req)
	assert.NoError(err)
	assert.Equal("Bearer token", strings.TrimSpace(req.Header.Get("Authorization")))

	// Decorate the a second time.
	_ = c.Decorate(req)

	c.Stop()

	// Multiple calls to Stop is ok.
	c.Stop()
}

func TestContextExpires(t *testing.T) {
	c, err := New(
		URL("http://example.com"),
		MacAddress(wrp.DeviceID("mac:112233445566")),
		SerialNumber("1234567890"),
		HardwareModel("model"),
		HardwareManufacturer("manufacturer"),
		FirmwareVersion("version"),
		LastRebootReason("reason"),
		XmidtProtocol("protocol"),
		BootRetryWait(1),
	)

	require.NoError(t, err)
	require.NotNil(t, c)

	ctx := context.Background()
	deadline, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()
	c.WaitUntilFetched(deadline)

	deadline, cancel = context.WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()
	c.WaitUntilValid(deadline)

	deadline, cancel = context.WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()
	c.MarkInvalid(deadline)
}

func TestDecorate(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				r.Body.Close()

				_, _ = w.Write([]byte(``))
			},
		),
	)
	defer server.Close()

	var count int
	c, err := New(
		URL(server.URL),
		MacAddress(wrp.DeviceID("mac:112233445566")),
		SerialNumber("1234567890"),
		HardwareModel("model"),
		HardwareManufacturer("manufacturer"),
		FirmwareVersion("version"),
		LastRebootReason("reason"),
		XmidtProtocol("protocol"),
		BootRetryWait(1),
		AddFetchListener(event.FetchListenerFunc(
			func(e event.Fetch) {
				assert.NoError(e.Err)
			})),
		AddDecorateListener(event.DecorateListenerFunc(
			func(e event.Decorate) {
				switch count {
				case 0:
					assert.ErrorIs(e.Err, ErrNilRequest)
				case 1:
					assert.ErrorIs(e.Err, ErrNoToken)
				default:
					assert.Fail("too many calls to decorate")
				}
				count++
			})),
	)

	require.NoError(err)
	require.NotNil(c)

	c.Start()
	defer c.Stop()

	ctx := context.Background()
	deadline, cancel := context.WithDeadline(ctx, time.Now().Add(1*time.Second))
	defer cancel()
	c.WaitUntilFetched(deadline)

	err = c.Decorate(nil)
	assert.ErrorIs(err, ErrNilRequest)

	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	err = c.Decorate(req)
	assert.ErrorIs(err, ErrNoToken)

	assert.Equal(2, count)
}

func TestEndToEndWithJwtPayload(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	when := time.Date(2023, 10, 30, 7, 4, 26, 0, time.UTC)

	token := `eyJhbGciOiJSUzI1NiIsImtpZCI6InRoZW1pcy0yMDE3MDEiLCJ0eXAiOiJKV1QifQ.` +
		`eyJhdWQiOiJYTWlEVCIsImNhcGFiaWxpdGllcyI6WyJ4MTppc3N1ZXI6dGVzdDouKjphbGwi` +
		`XSwiY3VzdG9tIjoicmJsIiwiZXhwIjoxNjk4Njc0NjY2LCJpYXQiOjE2OTYwODI2NjYsImlz` +
		`cyI6InRoZW1pcyIsImp0aSI6IldUZDh3SlV0Rzc3SkNZd3lWelRxRnciLCJtYWMiOiIxMTIy` +
		`MzM0NDU1NjYiLCJuYmYiOjE2OTYwODI1MTYsInBhcnRuZXItaWQiOiJjb21jYXN0Iiwic2Vy` +
		`aWFsIjoiMTIzNDU2Nzg5MCIsInN1YiI6ImNsaWVudDpzdXBwbGllZCIsInRydXN0IjoxMDAw` +
		`LCJ1dWlkIjoiMTczYTZlMjQtODgxOC00Nzk2LTgzNzYtNzdiOTA0NmJhZmVjIn0.invalid`

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				r.Body.Close()

				w.Header().Add("Expires", when.Format(http.TimeFormat))
				_, _ = w.Write([]byte(token))
			},
		),
	)
	defer server.Close()

	c, err := New(
		URL(server.URL),
		MacAddress(wrp.DeviceID("mac:112233445566")),
		SerialNumber("1234567890"),
		HardwareModel("model"),
		HardwareManufacturer("manufacturer"),
		FirmwareVersion("version"),
		LastRebootReason("reason"),
		XmidtProtocol("protocol"),
		BootRetryWait(1),
		AddFetchListener(event.FetchListenerFunc(
			func(e event.Fetch) {
				assert.Equal(when.Format(http.TimeFormat), e.Expiration.Format(http.TimeFormat))
				assert.NoError(e.Err)
			})),
	)

	require.NoError(err)
	require.NotNil(c)

	c.Start()
	defer c.Stop()

	ctx := context.Background()
	deadline, cancel := context.WithDeadline(ctx, time.Now().Add(1*time.Second))
	defer cancel()
	c.WaitUntilValid(deadline)
}
