// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type PriorityType int

const (
	UnknownType PriorityType = iota
	OldestType
	NewestType
	lastType
)

var ErrPriorityTypeInvalid = errors.New("Priority type is invalid")

var (
	PriorityTypeUnmarshal = map[string]PriorityType{
		"unknown": UnknownType,
		"oldest":  OldestType,
		"newest":  NewestType,
	}
	PriorityTypeMarshal = map[PriorityType]string{
		UnknownType: "unknown",
		OldestType:  "oldest",
		NewestType:  "newest",
	}
)

// String returns a human-readable string representation for an existing PriorityType,
// otherwise String returns the `unknown` string value.
func (pt PriorityType) String() string {
	if value, ok := PriorityTypeMarshal[pt]; ok {
		return value
	}

	return PriorityTypeMarshal[UnknownType]
}

// UnmarshalText unmarshals a PriorityType's enum value.
func (pt *PriorityType) UnmarshalText(b []byte) error {
	s := strings.ToLower(string(b))
	r, ok := PriorityTypeUnmarshal[s]
	if !ok {
		return errors.Join(ErrPriorityTypeInvalid, fmt.Errorf("PriorityType error: '%s' does not match any valid options: %s",
			s, pt.getKeys()))
	}

	*pt = r
	return nil
}

// getKeys returns the string keys for the PriorityType enums.
func (pt PriorityType) getKeys() string {
	keys := make([]string, 0, len(PriorityTypeUnmarshal))
	for k := range PriorityTypeUnmarshal {
		k = "'" + k + "'"
		keys = append(keys, k)
	}

	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
