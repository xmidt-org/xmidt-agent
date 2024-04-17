// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package loglevel

import (
	"time"

	"go.uber.org/zap"
)

type LogLevelService struct {
	level zap.AtomicLevel
	origLevel []byte
}

func New(level zap.AtomicLevel) (*LogLevelService, error) {
	origLevel, err := level.MarshalText()
	if (err != nil) {
		return nil, err
	}

	return &LogLevelService{
		level: level,
		origLevel: origLevel,
	}, nil
}

func (l *LogLevelService) SetLevel(level string, duration time.Duration) {
	l.level.UnmarshalText([]byte(level))

	t := time.NewTimer(duration)

	go func() {
        <-t.C
        l.level.UnmarshalText(l.origLevel)
    }()
}