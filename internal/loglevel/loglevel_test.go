// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package loglevel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/sallust"
)

func TestSetLevel(t *testing.T) {
	cfg := sallust.Config{
		Level: "ERROR",
	}

	zcfg, err := cfg.NewZapConfig()
	assert.NoError(t, err)

	_, err = cfg.Build()
	assert.NoError(t, err)

	level := zcfg.Level

	logLevel, err := New(level)
	assert.NoError(t, err)
	logLevelService := logLevel.(*LogLevelService)
	assert.Equal(t, "ERROR", logLevelService.level.Level().CapitalString())

	err = logLevelService.SetLevel("some-nonsense", 1*time.Second)
	assert.NotNil(t, err)
	assert.Equal(t, "ERROR", logLevelService.level.Level().CapitalString())

	err = logLevelService.SetLevel("DEBUG", 1*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, "DEBUG", logLevelService.level.Level().CapitalString())

	time.Sleep(2 * time.Second)

	assert.Equal(t, "ERROR", logLevelService.level.Level().CapitalString())
}
