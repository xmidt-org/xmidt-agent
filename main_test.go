// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/goschtalt/goschtalt"
	_ "github.com/goschtalt/goschtalt/pkg/typical"
	_ "github.com/goschtalt/yaml-decoder"
	_ "github.com/goschtalt/yaml-encoder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/sallust"
)

func Test_handleCLIShow(t *testing.T) {
	gs, err := goschtalt.New()
	require.NoError(t, err)
	require.NotNil(t, gs)

	tests := []struct {
		description string
		cli         *CLI
		cfg         *goschtalt.Config
		expectEarly bool
	}{
		{
			description: "early exit",
			cli: &CLI{
				Show: true,
			},
			cfg:         gs,
			expectEarly: true,
		}, {
			description: "no early exit",
			cli:         &CLI{},
			cfg:         gs,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			var early earlyExit
			handleCLIShow(tc.cli, tc.cfg, &early)

			assert.Equal(tc.expectEarly, bool(early))
		})
	}
}

func Test_provideCLI(t *testing.T) {
	tests := []struct {
		description string
		args        cliArgs
		earlyExit   bool
		dev         bool
		want        CLI
		expectedErr error
	}{
		{
			description: "no arguments, everything works",
		}, {
			description: "dev mode",
			args:        cliArgs{"-d"},
			dev:         true,
			want:        CLI{Dev: true},
		}, {
			description: "invalid argument",
			args:        cliArgs{"-w"},
			earlyExit:   true,
		}, {
			description: "invalid argument",
			args:        cliArgs{"-d", "-w"},
			earlyExit:   true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			var devMode devMode
			var earlyExit earlyExit
			got, err := provideCLI(tc.args, &devMode, &earlyExit)

			assert.ErrorIs(err, tc.expectedErr)
			want := tc.want
			assert.Equal(&want, got)
			assert.Equal(tc.earlyExit, bool(earlyExit))
			assert.Equal(tc.dev, bool(devMode))
		})
	}
}

func Test_xmidtAgent(t *testing.T) {
	tests := []struct {
		description string
		args        []string
		expectedErr error
	}{
		{
			description: "show config and exit",
			args:        []string{"-s"},
		},
		{
			description: "show help and exit",
			args:        []string{"-h"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			err := xmidtAgent(tc.args)

			assert.ErrorIs(err, tc.expectedErr)
		})
	}
}

func Test_provideLogger(t *testing.T) {
	tests := []struct {
		description string
		cli         *CLI
		cfg         sallust.Config
		expectedErr error
	}{
		{
			description: "validate empty config",
			cfg:         sallust.Config{},
			cli:         &CLI{},
		}, {
			description: "validate dev config",
			cfg:         sallust.Config{},
			cli:         &CLI{Dev: true},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			got, err := provideLogger(tc.cli, tc.cfg)

			if tc.expectedErr == nil {
				assert.NotNil(got)
				assert.NoError(err)
				return
			}
			assert.ErrorIs(err, tc.expectedErr)
			assert.Nil(got)
		})
	}
}
