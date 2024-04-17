// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/goschtalt/goschtalt"
	_ "github.com/goschtalt/properties-decoder"
	_ "github.com/goschtalt/yaml-decoder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mapping struct {
	in    string
	out   string
	found bool
}

func TestExternal_resolve(t *testing.T) {
	unknownErr := errors.New("unknown error")

	testFs := fstest.MapFS{
		"one.txt": &fstest.MapFile{
			Data: []byte(`
Device.Some.Thing.Something.Else=red
Device.Other.Thing.Something.Else=green
Device.URL=https://fabric.xmidt.example.com
`,
			),
			Mode: 0755,
		},
	}

	tests := []struct {
		description string
		in          External
		tests       []mapping
		expectedErr error
	}{
		{
			description: "empty external",
			tests: []mapping{
				{
					in:    "test",
					out:   "",
					found: false,
				},
			},
		}, {
			description: "most cases for external",
			in: External{
				Required: true,
				File:     "one.txt",
				As:       "properties",
				Remap: []Remap{
					{
						From: "Device.Some.Thing.Something.Else",
						To:   "Item1",
					}, {
						From: "Device.Other.Thing.Something.Else",
						To:   "Item2",
					}, {
						From: "Device.URL",
						To:   "URL",
					}, {
						From:     "Not.There.But.Optional",
						To:       "missing",
						Optional: true,
					},
				},
				root: testFs,
			},
			tests: []mapping{
				{
					in:    "test",
					out:   "",
					found: false,
				}, {
					in:    "Item1",
					out:   "red",
					found: true,
				}, {
					in:    "Item2",
					out:   "green",
					found: true,
				}, {
					in:    "URL",
					out:   "https://fabric.xmidt.example.com",
					found: true,
				}, {
					in:    "missing",
					found: false,
				},
			},
		}, {
			description: "required, but not there",
			in: External{
				Required: true,
				File:     "invalid.file",
				root:     testFs,
			},
			expectedErr: unknownErr,
		}, {
			description: "required field but not there",
			expectedErr: unknownErr,
			in: External{
				Required: true,
				File:     "one.txt",
				As:       "properties",
				Remap: []Remap{
					{
						From: "Not.There",
						To:   "Item1",
					},
				},
				root: testFs,
			},
		}, {
			description: "invalid remap option",
			in: External{
				Required: true,
				File:     "one.txt",
				As:       "properties",
				Remap: []Remap{
					{
						From:     "", // missing/invalid but ok
						To:       "Item1",
						Optional: true,
					}, {
						From: "", // missing/invalid but not ok
						To:   "Item2",
					}, {
						From: "Device.URL",
						To:   "URL",
					},
				},
				root: testFs,
			},
			expectedErr: unknownErr,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			got, err := tc.in.resolve()

			if tc.expectedErr != nil {
				assert.Error(err)
				if !errors.Is(tc.expectedErr, unknownErr) {
					assert.ErrorIs(err, tc.expectedErr)
				}
				return
			}

			assert.NoError(err)
			assert.NotNil(got)

			for _, test := range tc.tests {
				out, found := got(test.in)
				assert.Equal(test.found, found)
				assert.Equal(test.out, out)
			}
		})
	}
}

func TestExternal_apply(t *testing.T) {
	unknownErr := errors.New("unknown error")

	testFs := fstest.MapFS{
		"cfg.yaml": &fstest.MapFile{
			Data: []byte(`
---
  value: ${URL}
  externals:
    - file: one.txt
      as: properties
      remap:
        - from: Device.Some.Thing.Something.Else
          to: Item1
        - from: Device.Other.Thing.Something.Else
          to: Item2
        - from: Device.URL
          to: URL
  invalid:
    - file: one.txt
      as: properties
      remap:
        - from: Device.URL # missing the to
`,
			),
			Mode: 0755,
		},
		"one.txt": &fstest.MapFile{
			Data: []byte(`
Device.Some.Thing.Something.Else=red
Device.Other.Thing.Something.Else=green
Device.URL=https://fabric.xmidt.example.com
`,
			),
			Mode: 0755,
		},
	}

	tests := []struct {
		description string
		name        string
		fs          fs.FS
		expectedErr error
		required    bool
	}{
		{
			description: "a missing external, but not required",
			name:        "missing",
			fs:          testFs,
		}, {
			description: "externals are present",
			name:        "externals",
			fs:          testFs,
		}, {
			description: "a missing external, but required",
			name:        "missing",
			fs:          testFs,
			required:    true,
			expectedErr: unknownErr,
		}, {
			description: "a remap, but required",
			name:        "invalid",
			fs:          testFs,
			required:    true,
			expectedErr: unknownErr,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			gs, err := goschtalt.New(
				goschtalt.AddFiles(tc.fs, "."),
				goschtalt.ConfigIs("two_words"),
				goschtalt.AutoCompile(true),
			)
			require.NoError(err)
			require.NotNil(gs)

			got := apply(gs, tc.name, tc.required, tc.fs)

			if tc.expectedErr != nil {
				assert.Error(got)
				if !errors.Is(tc.expectedErr, unknownErr) {
					assert.ErrorIs(got, tc.expectedErr)
				}
				return
			}

			assert.NoError(got)
		})
	}
}
