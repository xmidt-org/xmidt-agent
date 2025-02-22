// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrpkit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/wrp-go/v5"
)

func TestHandlerFunc_HandleWrp(t *testing.T) {
	assert := assert.New(t)

	h := HandlerFunc(func(wrp.Message) error {
		return nil
	})

	msg := wrp.Message{}

	err := h.HandleWrp(msg)

	assert.NoError(err)
}
