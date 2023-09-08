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
