// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandler_PriorityType(t *testing.T) {
	unknown := PriorityType(-10)
	assert.New(t).Equal("unknown", unknown.String())

	tests := []struct {
		description   string
		config        string
		expectedType  PriorityType
		expectedError error
	}{
		{
			description:  "unknown type",
			config:       "unknown",
			expectedType: UnknownType,
		},
		{
			description:  "oldest type",
			config:       "oldest",
			expectedType: OldestType,
		},
		{
			description:  "newest type",
			config:       "newest",
			expectedType: NewestType,
		},
		{
			description:   "unknown random type error",
			config:        "DEADBEEF_RANDOM",
			expectedError: ErrPriorityTypeInvalid,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			var pt PriorityType
			err := pt.UnmarshalText([]byte(tc.config))
			if tc.expectedError != nil {
				assert.ErrorIs(err, tc.expectedError)
				return
			}

			assert.NoError(err)
			assert.Equal(tc.config, pt.String())
			assert.NotEmpty(pt.getKeys())
		})
	}
}
