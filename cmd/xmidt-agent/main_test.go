// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"
	"time"

	_ "github.com/goschtalt/goschtalt/pkg/typical"
	_ "github.com/goschtalt/yaml-decoder"
	_ "github.com/goschtalt/yaml-encoder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmidt-org/sallust"
)

func Test_provideCLI(t *testing.T) {
	tests := []struct {
		description string
		args        cliArgs
		want        CLI
		exits       bool
		expectedErr error
	}{
		{
			description: "no arguments, everything works",
		}, {
			description: "dev mode",
			args:        cliArgs{"-d"},
			want:        CLI{Dev: true},
		}, {
			description: "invalid argument",
			args:        cliArgs{"-w"},
			exits:       true,
		}, {
			description: "invalid argument",
			args:        cliArgs{"-d", "-w"},
			exits:       true,
		}, {
			description: "help",
			args:        cliArgs{"-h"},
			exits:       true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			if tc.exits {
				assert.Panics(func() {
					_, _ = provideCLIWithOpts(tc.args, true)
				})
			} else {
				got, err := provideCLI(tc.args)

				assert.ErrorIs(err, tc.expectedErr)
				want := tc.want
				assert.Equal(&want, got)
			}
		})
	}
}

func Test_xmidtAgent(t *testing.T) {
	tests := []struct {
		description string
		args        []string
		duration    time.Duration
		expectedErr error
		panic       bool
	}{
		{
			description: "show config and exit",
			args:        []string{"-s"},
			panic:       true,
		}, {
			description: "show help and exit",
			args:        []string{"-h"},
			panic:       true,
		}, {
			description: "confirm invalid config file check works",
			args:        []string{"-f", "invalid.yml"},
			panic:       true,
		}, {
			description: "enable debug mode",
			args:        []string{"-d", "-f", "xmidt_agent.yaml"},
		}, {
			description: "output graph",
			args:        []string{"-g", "graph.dot", "-f", "xmidt_agent.yaml"},
		}, {
			description: "start and stop",
			duration:    time.Millisecond,
			args:        []string{"-f", "xmidt_agent.yaml"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			if tc.panic {
				assert.Panics(func() {
					_, _ = xmidtAgent(tc.args)
				})
				return
			}

			app, err := xmidtAgent(tc.args)

			assert.ErrorIs(err, tc.expectedErr)
			if tc.expectedErr != nil {
				assert.Nil(app)
				return
			} else {
				require.NoError(err)
			}

			if tc.duration <= 0 {
				return
			}

			// only run the program for	a few seconds to make sure it starts
			startCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err = app.Start(startCtx)
			require.NoError(err)

			time.Sleep(tc.duration)

			stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err = app.Stop(stopCtx)
			require.NoError(err)
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

			level, got, err := provideLogger(LoggerIn{CLI: tc.cli, Cfg: tc.cfg})

			if tc.expectedErr == nil {
				assert.NotNil(got)
				assert.NotNil(level)
				assert.NoError(err)
				return
			}
			assert.ErrorIs(err, tc.expectedErr)
			assert.Nil(got)
		})
	}
}
