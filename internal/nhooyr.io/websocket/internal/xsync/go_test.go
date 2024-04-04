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
