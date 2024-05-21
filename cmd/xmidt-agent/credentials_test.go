// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/xmidt-agent/internal/credentials"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func Test_provideCredentials(t *testing.T) {
	tests := []struct {
		description string
		in          credsIn
		want        bool
		wantErr     bool
		checkLog    func(assert *assert.Assertions, logs []observer.LoggedEntry)
	}{}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			core, logs := observer.New(zap.InfoLevel)

			tc.in.Logger = zap.New(core)

			got, err := provideCredentials(tc.in)

			if tc.checkLog != nil {
				tc.checkLog(assert, logs.AllUntimed())
			}

			if tc.wantErr {
				assert.Error(err)
				assert.Nil(got)
				return
			}

			assert.NoError(err)
			if tc.want {
				assert.NotNil(got)
			} else {
				assert.Nil(got)
			}
		})
	}
}

func Test_credsIn_Options(t *testing.T) {
	tests := []struct {
		description string
		in          credsIn
		want        []credentials.Option
		checkLog    func(assert *assert.Assertions, logs []observer.LoggedEntry)
		wantErr     bool
	}{
		{
			description: "No credentials service configured",
			checkLog: func(assert *assert.Assertions, logs []observer.LoggedEntry) {
				assert.Len(logs, 1)
				assert.Equal(zap.WarnLevel, logs[0].Level)
				assert.Equal("no credentials service configured", logs[0].Message)
			},
		},
		{
			description: "Mostly empty, but valid",
			in: credsIn{
				Creds: XmidtCredentials{
					URL: "http://example.com",
				},
			},
			want: []credentials.Option{
				credentials.URL("http://example.com"),
				credentials.MacAddress("mac:112233445566"),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			core, logs := observer.New(zap.InfoLevel)
			tc.in.Logger = zap.New(core)

			got, err := tc.in.Options()

			if tc.checkLog != nil {
				tc.checkLog(assert, logs.AllUntimed())
			}

			if tc.wantErr {
				assert.Error(err)
				assert.Nil(got)
				return
			}

			assert.NoError(err)
			//assert.Equal(tc.want, got)
		})
	}
}
