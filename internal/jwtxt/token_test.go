// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package jwtxt

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/foxcpp/go-mockdns"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/xmidt-agent/internal/jwtxt/event"
)

func randomResolver() Option {
	return UseResolver(&mockdns.Resolver{
		Zones: map[string]mockdns.Zone{
			"112233445566.fabric.random.example.org.": {
				TXT: []string{
					"01:eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbmRwb2ludCI6I",
					"02:mZhYnJpYy54bWlkdC5leGFtcGxlLm9yZyIsImV4cCI6MTY5MDAwMDA",
					"03:wMH0.4ELQaJAcX67M0Me1ZjTAusZT3QZpiCj2WQATDCvgllnEN9g4R",
					"04:xMeDqnqnYAE_GdzsXI_e9fAGI9o1QuIym7_zQ",
				},
			},
		},
	})
}

type niceNeverResolver struct{}

func (niceNeverResolver) LookupTXT(ctx context.Context, _ string) ([]string, error) {
	<-ctx.Done()

	return nil, &net.DNSError{
		Err:       "context canceled",
		IsTimeout: true,
	}
}

type notNiceNeverResolver struct{}

func (notNiceNeverResolver) LookupTXT(ctx context.Context, _ string) ([]string, error) {
	<-ctx.Done()

	return nil, errors.New("context canceled")
}

func TestInstructions_EndToEnd(t *testing.T) {
	unknownErr := errors.New("unknown")
	tests := []struct {
		description         string
		opts                []Option
		times               []int64
		listener            func(*assert.Assertions, event.Fetch)
		callEndpointTwice   bool
		expectedEndpoint    string
		expectedNewErr      error
		expectedEndpointErr error
	}{
		{
			description: "simple successful match",
			times:       []int64{1680000000},
			opts: []Option{
				BaseURL("https://fabric.random.example.org"),
				DeviceID("mac:112233445566"),
				Algorithms("ES256"),
				publicECOption(),
				randomResolver(),
			},
			listener: func(assert *assert.Assertions, fe event.Fetch) {
				assert.Equal("112233445566.fabric.random.example.org", fe.FQDN)
				assert.Equal("", fe.Server)
				assert.True(fe.Found)
				assert.False(fe.Timeout)
				assert.Equal(time.Time{}, fe.PriorExpiration)
				assert.Equal(time.Unix(1690000000, 0), fe.Expiration)
				assert.False(fe.TemporaryErr)
				assert.Equal("fabric.xmidt.example.org", fe.Endpoint)
				assert.Equal(`{"endpoint":"fabric.xmidt.example.org","exp":1690000000}`, string(fe.Payload))
				assert.NoError(fe.Err)
			},
			expectedEndpoint: "fabric.xmidt.example.org",
		}, {
			description: "simple successful match returned twice",
			times:       []int64{1680000000},
			opts: []Option{
				BaseURL("https://fabric.random.example.org"),
				DeviceID("mac:112233445566"),
				Algorithms("ES256"),
				publicECOption(),
				randomResolver(),
			},
			listener: func(assert *assert.Assertions, fe event.Fetch) {
				assert.Equal("112233445566.fabric.random.example.org", fe.FQDN)
				assert.Equal("", fe.Server)
				assert.True(fe.Found)
				assert.False(fe.Timeout)
				assert.Equal(time.Time{}, fe.PriorExpiration)
				assert.Equal(time.Unix(1690000000, 0), fe.Expiration)
				assert.False(fe.TemporaryErr)
				assert.Equal("fabric.xmidt.example.org", fe.Endpoint)
				assert.Equal(`{"endpoint":"fabric.xmidt.example.org","exp":1690000000}`, string(fe.Payload))
				assert.NoError(fe.Err)
			},
			callEndpointTwice: true,
			expectedEndpoint:  "fabric.xmidt.example.org",
		}, {
			description: "successful match starting at 0, with empty lines and out of order",
			times:       []int64{1680000000},
			opts: []Option{
				BaseURL("https://fabric.random.example.org"),
				DeviceID("mac:112233445566"),
				Algorithms("ES256"),
				publicECOption(),
				UseResolver(&mockdns.Resolver{
					Zones: map[string]mockdns.Zone{
						"112233445566.fabric.random.example.org.": {
							TXT: []string{
								"01:mZhYnJpYy54bWlkdC5leGFtcGxlLm9yZyIsImV4cCI6MTY5MDAwMDA",
								"",
								"ignored:value",
								"02:wMH0.4ELQaJAcX67M0Me1ZjTAusZT3QZpiCj2WQATDCvgllnEN9g4R",
								"ignored:value:somewhere_else",
								"03:xMeDqnqnYAE_GdzsXI_e9fAGI9o1QuIym7_zQ",
								"00:eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbmRwb2ludCI6I",
							},
						},
					},
				}),
			},
			expectedEndpoint: "fabric.xmidt.example.org",
		}, {
			description:         "no record",
			times:               []int64{1680000000},
			expectedEndpointErr: unknownErr,
			opts: []Option{
				BaseURL("https://fabric.random.example.org"),
				DeviceID("mac:112233445566"),
				Algorithms("ES256"),
				publicECOption(),
				UseResolver(&mockdns.Resolver{}),
			},
			listener: func(assert *assert.Assertions, fe event.Fetch) {
				assert.Equal("112233445566.fabric.random.example.org", fe.FQDN)
				assert.False(fe.Found)
				assert.False(fe.Timeout)
				assert.Equal(time.Time{}, fe.PriorExpiration)
				assert.Equal(time.Time{}, fe.Expiration)
				assert.False(fe.TemporaryErr)
				assert.Equal("", fe.Endpoint)
				assert.Error(fe.Err)
			},
		}, {
			description:         "missing segments",
			times:               []int64{1680000000},
			expectedEndpointErr: jwt.ErrTokenMalformed,
			opts: []Option{
				BaseURL("https://fabric.random.example.org"),
				DeviceID("mac:112233445566"),
				Algorithms("ES256"),
				publicECOption(),
				UseResolver(&mockdns.Resolver{
					Zones: map[string]mockdns.Zone{
						"112233445566.fabric.random.example.org.": {
							TXT: []string{

								"00:eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbmRwb2ludCI6I",
								"01:mZhYnJpYy54bWlkdC5leGFtcGxlLm9yZyIsImV4cCI6MTY5MDAwMDA",
								"03:xMeDqnqnYAE_GdzsXI_e9fAGI9o1QuIym7_zQ",
							},
						},
					},
				}),
			},
		}, {
			description:         "expired token",
			times:               []int64{1700000000},
			expectedEndpointErr: jwt.ErrTokenExpired,
			opts: []Option{
				BaseURL("https://fabric.random.example.org"),
				DeviceID("mac:112233445566"),
				Algorithms("ES256"),
				publicECOption(),
				randomResolver(),
			},
			listener: func(assert *assert.Assertions, fe event.Fetch) {
				assert.Equal("112233445566.fabric.random.example.org", fe.FQDN)
				assert.True(fe.Found)
				assert.False(fe.Timeout)
				assert.Equal(time.Time{}, fe.PriorExpiration)
				assert.Equal(time.Time{}, fe.Expiration)
				assert.False(fe.TemporaryErr)
				assert.Equal("", fe.Endpoint)
				assert.ErrorIs(fe.Err, jwt.ErrTokenExpired)
			},
		}, {
			description:         "times out with nice resolver",
			times:               []int64{1680000000},
			expectedEndpointErr: unknownErr,
			opts: []Option{
				BaseURL("https://fabric.random.example.org"),
				DeviceID("mac:112233445566"),
				Algorithms("ES256"),
				publicECOption(),
				UseResolver(&niceNeverResolver{}),
				Timeout(time.Nanosecond),
			},
			listener: func(assert *assert.Assertions, fe event.Fetch) {
				assert.Equal("112233445566.fabric.random.example.org", fe.FQDN)
				assert.False(fe.Found)
				assert.True(fe.Timeout)
				assert.Equal(time.Time{}, fe.PriorExpiration)
				assert.Equal(time.Time{}, fe.Expiration)
				assert.True(fe.TemporaryErr)
				assert.Empty(fe.Endpoint)
				assert.Error(fe.Err)
			},
		}, {
			description:         "times out with not nice resolver",
			times:               []int64{1680000000},
			expectedEndpointErr: unknownErr,
			opts: []Option{
				BaseURL("https://fabric.random.example.org"),
				DeviceID("mac:112233445566"),
				Algorithms("ES256"),
				publicECOption(),
				UseResolver(&notNiceNeverResolver{}),
				Timeout(time.Nanosecond),
			},
			listener: func(assert *assert.Assertions, fe event.Fetch) {
				assert.Equal("112233445566.fabric.random.example.org", fe.FQDN)
				assert.False(fe.Found)
				assert.True(fe.Timeout)
				assert.Equal(time.Time{}, fe.PriorExpiration)
				assert.Equal(time.Time{}, fe.Expiration)
				assert.True(fe.TemporaryErr)
				assert.Empty(fe.Endpoint)
				assert.Error(fe.Err)
			},
		}, {
			description:         "no algorithms",
			times:               []int64{1680000000},
			expectedEndpointErr: jwt.ErrTokenSignatureInvalid,
			opts: []Option{
				BaseURL("https://fabric.random.example.org"),
				DeviceID("mac:112233445566"),
				anyPublicOption(),
				randomResolver(),
			},
		}, {
			description:         "no keys",
			times:               []int64{1680000000},
			expectedEndpointErr: jwt.ErrTokenUnverifiable,
			opts: []Option{
				BaseURL("https://fabric.random.example.org"),
				DeviceID("mac:112233445566"),
				Algorithms("ES256"),
				randomResolver(),
			},
		}, {
			description:    "no device id",
			expectedNewErr: ErrInvalidInput,
			opts: []Option{
				BaseURL("https://fabric.random.example.org"),
				Algorithms("ES256"),
				publicECOption(),
				randomResolver(),
			},
		}, {
			description:    "no base url",
			expectedNewErr: ErrInvalidInput,
			opts: []Option{
				DeviceID("mac:112233445566"),
				Algorithms("ES256"),
				publicECOption(),
				randomResolver(),
			},
		}, {
			description: "invalid base url",
			opts: []Option{
				BaseURL("invalid"),
			},
			expectedNewErr: ErrInvalidInput,
		}, {
			description: "invalid device id",
			opts: []Option{
				DeviceID("invalid"),
			},
			expectedNewErr: ErrInvalidInput,
		}, {
			description: "invalid algorithm",
			opts: []Option{
				Algorithms("invalid"),
			},
			expectedNewErr: ErrUnspportedAlg,
		}, {
			description: "invalid pem",
			opts: []Option{
				WithPEMs([]byte("invalid")),
			},
			expectedNewErr: ErrInvalidInput,
		}, {
			description: "invalid timeout",
			opts: []Option{
				Timeout(0), // ok, just set the default again.
				Timeout(-1),
			},
			expectedNewErr: ErrInvalidInput,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			then := func() time.Time { return time.Unix(tc.times[0], 0) }

			opts := append(tc.opts, UseNowFunc(then))
			if tc.listener != nil {
				opts = append(opts, WithFetchListener(
					event.FetchListenerFunc(
						func(fe event.Fetch) {
							tc.listener(assert, fe)
						},
					)))
			}
			obj, err := New(opts...)

			if tc.expectedNewErr == nil {
				assert.NoError(err)
				require.NotNil(obj)
			} else {
				assert.ErrorIs(err, tc.expectedNewErr)
				return
			}

			when := jwt.WithTimeFunc(then)
			obj.jwtOptions = append(obj.jwtOptions, when)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
			defer cancel()

			if tc.callEndpointTwice {
				endpoint, err := obj.Endpoint(ctx)
				require.NoError(err)
				require.NotEmpty(endpoint)
			}
			endpoint, err := obj.Endpoint(ctx)

			if tc.expectedEndpointErr != nil {
				assert.Error(err)
				assert.Empty(endpoint)

				if !errors.Is(tc.expectedEndpointErr, unknownErr) {
					assert.ErrorIs(err, tc.expectedEndpointErr)
				}
				return
			}

			assert.NoError(err)
			assert.Equal(tc.expectedEndpoint, endpoint)
		})
	}
}
