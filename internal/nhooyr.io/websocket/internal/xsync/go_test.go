// SPDX-FileCopyrightText: 2023 Anmol Sethi <hi@nhooyr.io>
// SPDX-License-Identifier: ISC

package xsync

import (
	"testing"

	"github.com/xmidt-org/xmidt-agent/internal/nhooyr.io/websocket/internal/test/assert"
)

func TestGoRecover(t *testing.T) {
	t.Parallel()

	errs := Go(func() error {
		panic("anmol")
	})

	err := <-errs
	assert.Contains(t, err, "anmol")
}
