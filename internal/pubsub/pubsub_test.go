// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package pubsub

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/wrp-go/v4"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

func TestNew(t *testing.T) {
	fn := wrpkit.HandlerFunc(func(wrp.Message) error {
		return nil
	})

	tests := []struct {
		description string
		self        string
		opt         Option
		opts        []Option
		expectedErr error
		validate    func(*assert.Assertions, *PubSub)
	}{
		{
			description: "Happy Path",
			self:        "mac:112233445566",
			validate: func(a *assert.Assertions, ps *PubSub) {
				a.Equal(wrp.DeviceID("mac:112233445566"), ps.self)
			},
		}, {
			description: "Confirm egress",
			self:        "mac:112233445566",
			opt:         WithEgressHandler(fn),
			validate: func(a *assert.Assertions, ps *PubSub) {
				a.NotNil(ps.routes["egress:*"])
			},
		}, {
			description: "Confirm local services",
			self:        "mac:112233445566",
			opts: []Option{
				WithServiceHandler("config", fn),
				WithServiceHandler("*", fn),
			},
			validate: func(a *assert.Assertions, ps *PubSub) {
				a.NotNil(ps.routes["service:*"])
				a.NotNil(ps.routes["service:config"])
			},
		}, {
			description: "Confirm event service",
			self:        "mac:112233445566",
			opts: []Option{
				WithEventHandler("*", fn),
				WithEventHandler("device-status", fn),
			},
			validate: func(a *assert.Assertions, ps *PubSub) {
				a.NotNil(ps.routes["event:*"])
				a.NotNil(ps.routes["event:device-status"])
			},
		}, {
			description: "Register multiple handlers",
			self:        "mac:112233445566",
			opts: []Option{
				WithEgressHandler(fn),
				WithEventHandler("*", fn),
				WithEventHandler("device-status", fn),
				WithServiceHandler("config", fn),
				WithServiceHandler("*", fn),
			},
			validate: func(a *assert.Assertions, ps *PubSub) {
				a.NotNil(ps.routes["egress:*"])
				a.NotNil(ps.routes["event:*"])
				a.NotNil(ps.routes["event:device-status"])
				a.NotNil(ps.routes["service:*"])
				a.NotNil(ps.routes["service:config"])
			},
		}, {
			description: "Confirm normify options",
			self:        "mac:112233445566",
			opts: []Option{
				Normify(wrp.EnsureTransactionUUID()),
			},
			validate: func(a *assert.Assertions, ps *PubSub) {
				a.NotNil(ps.desiredOpts)
				a.Equal(1, len(ps.desiredOpts))
			},
		},

		// Error Cases
		{
			description: "Empty Self",
			expectedErr: ErrInvalidInput,
		}, {
			description: "Empty Egress Handler",
			self:        "mac:112233445566",
			opts:        []Option{WithEgressHandler(nil)},
			expectedErr: ErrInvalidInput,
		}, {
			description: "Invalid Service Name",
			self:        "mac:112233445566",
			opts:        []Option{WithServiceHandler("foo/bar", fn)},
			expectedErr: ErrInvalidInput,
		}, {
			description: "Empty Service Name",
			self:        "mac:112233445566",
			opts:        []Option{WithServiceHandler("", fn)},
			expectedErr: ErrInvalidInput,
		}, {
			description: "Invalid Event Name",
			self:        "mac:112233445566",
			opts:        []Option{WithEventHandler("foo/bar", fn)},
			expectedErr: ErrInvalidInput,
		}, {
			description: "Empty Event Name",
			self:        "mac:112233445566",
			opts:        []Option{WithEventHandler("", fn)},
			expectedErr: ErrInvalidInput,
		}, {
			description: "Invalid timeout",
			self:        "mac:112233445566",
			opts:        []Option{WithPublishTimeout(-1 * time.Second)},
			expectedErr: ErrInvalidInput,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			opts := append(tc.opts, tc.opt)
			id := wrp.DeviceID(tc.self)

			got, err := New(id, opts...)

			if tc.expectedErr != nil {
				assert.ErrorIs(err, tc.expectedErr)
				assert.Nil(got)
				return
			}

			assert.NoError(err)
			require.NotNil(got)
			if tc.validate != nil {
				tc.validate(assert, got)
			}
		})
	}
}
