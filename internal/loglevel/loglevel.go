// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package loglevel

import (
	"strings"
	"time"

	"go.uber.org/zap"
)

type LogLevel interface {
	SetLevel(string, time.Duration) error
}

type LogLevelService struct {
	level     *zap.AtomicLevel
	origLevel []byte
}

func New(level *zap.AtomicLevel) (LogLevel, error) {
	origLevel, err := level.MarshalText()
	if err != nil {
		return nil, err
	}

	return &LogLevelService{
		level:     level,
		origLevel: origLevel,
	}, nil
}

// note that zap will set log level to "INFO" if level is empty
func (l *LogLevelService) SetLevel(level string, duration time.Duration) error {
	level = strings.ToLower(level)

	err := l.level.UnmarshalText([]byte(level))
	if err != nil {
		return err
	}

	t := time.NewTimer(duration)

	go func() {
		<-t.C
		l.level.UnmarshalText(l.origLevel)
	}()

	return nil
}
