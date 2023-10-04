// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package fs_test

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	xafs "github.com/xmidt-org/xmidt-agent/internal/fs"
	"github.com/xmidt-org/xmidt-agent/internal/fs/mem"
)

var (
	errUnknown = errors.New("unknown error")
)

func TestWithDirOrSimiar(t *testing.T) {
	tests := []struct {
		description string
		opt         xafs.Option
		opts        []xafs.Option
		start       *mem.FS
		expect      *mem.FS
		expectErr   error
	}{
		{
			description: "simple path",
			opt:         xafs.WithDir("foo", 0755),
			expect:      mem.New(mem.WithDir("foo", 0755)),
		}, {
			description: "simple existing path",
			opt:         xafs.WithDir("foo", 0755),
			start:       mem.New(mem.WithDir("foo", 0755)),
			expect:      mem.New(mem.WithDir("foo", 0755)),
		}, {
			description: "not a directory",
			opt:         xafs.WithDir("foo", 0755),
			start:       mem.New(mem.WithFile("foo", "data", 0755)),
			expectErr:   xafs.ErrNotDirectory,
		}, {
			description: "error opening the file",
			opt:         xafs.WithDir("foo", 0755),
			start:       mem.New(mem.WithError("foo", errUnknown)),
			expectErr:   errUnknown,
		}, {
			description: "three directory path",
			opts: []xafs.Option{
				xafs.WithDir("foo", 0700),
				xafs.WithDir("foo/bar", 0750),
				xafs.WithDir("foo/bar/car", 0755),
			},
			expect: mem.New(
				mem.WithDir("foo", 0700),
				mem.WithDir("foo/bar", 0750),
				mem.WithDir("foo/bar/car", 0755),
			),
		}, {
			description: "abs directory path",
			start:       mem.New(mem.WithDir("/", 0755)),
			opts: []xafs.Option{
				xafs.WithDir("/foo", 0700),
				xafs.WithDir("/foo/bar", 0750),
				xafs.WithDir("/foo/bar/car", 0755),
			},
			expect: mem.New(
				mem.WithDir("/", 0755),
				mem.WithDir("/foo", 0700),
				mem.WithDir("/foo/bar", 0750),
				mem.WithDir("/foo/bar/car", 0755),
			),
		}, {
			description: "WithDirs three directory path",
			start:       mem.New(),
			opts: []xafs.Option{
				xafs.WithDirs("foo/bar/car", 0755),
			},
			expect: mem.New(
				mem.WithDir("foo", 0755),
				mem.WithDir("foo/bar", 0755),
				mem.WithDir("foo/bar/car", 0755),
			),
		}, {
			description: "WithDirs three directory path one exists",
			start:       mem.New(mem.WithDir("foo", 0711)),
			opts: []xafs.Option{
				xafs.WithDirs("foo/bar/car", 0755),
			},
			expect: mem.New(
				mem.WithDir("foo", 0711),
				mem.WithDir("foo/bar", 0755),
				mem.WithDir("foo/bar/car", 0755),
			),
		}, {
			description: "abs three directory path",
			start:       mem.New(mem.WithDir("/", 0700)),
			opts: []xafs.Option{
				xafs.WithDirs("/boo/egg/cat", 0755),
			},
			expect: mem.New(
				mem.WithDir("/", 0700),
				mem.WithDir("/boo", 0755),
				mem.WithDir("/boo/egg", 0755),
				mem.WithDir("/boo/egg/cat", 0755),
			),
		}, {
			description: "WithPath two directory path one exists, and a filename",
			start:       mem.New(mem.WithDir("foo", 0711)),
			opts: []xafs.Option{
				xafs.WithPath("foo/bar/car.json", 0755),
			},
			expect: mem.New(
				mem.WithDir("foo", 0711),
				mem.WithDir("foo/bar", 0755),
			),
		}, {
			description: "Ensure Operate can handle nil options",
			start:       mem.New(mem.WithDir("foo", 0711)),
			opts: []xafs.Option{
				nil,
				xafs.WithPath("foo/bar/car.json", 0755),
			},
			expect: mem.New(
				mem.WithDir("foo", 0711),
				mem.WithDir("foo/bar", 0755),
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)

			opts := make([]xafs.Option, 0, len(tc.opts)+1)
			if tc.opt != nil {
				opts = append(tc.opts, tc.opt)
			}
			opts = append(opts, tc.opts...)

			fs := tc.start
			if fs == nil {
				fs = mem.New()
			}

			last := opts[len(opts)-1]
			opts = opts[:len(opts)-1]

			err := xafs.Operate(fs, opts...)
			require.NoError(err)

			err = xafs.Operate(fs, last)

			assert.ErrorIs(err, tc.expectErr)

			if tc.expectErr == nil {
				assert.Equal(tc.expect, fs)
			}
		})
	}
}

func TestReadFileWithSHA256(t *testing.T) {
	tests := []struct {
		description string
		filename    string
		start       *mem.FS
		expect      string
		expectErr   error
	}{
		{
			description: "simple path",
			filename:    "./foo",
			start: mem.New(
				mem.WithDir(".", 0755),
				mem.WithFile("./foo", "text\n", 0644),
				mem.WithFile("./foo.sha256", "b9e68e1bea3e5b19ca6b2f98b73a54b73daafaa250484902e09982e07a12e733  foo\n", 0644),
			),
			expect: "text\n",
		}, {
			description: "no ./ prefix",
			filename:    "foo",
			start: mem.New(
				mem.WithDir(".", 0755),
				mem.WithFile("foo", "text\n", 0644),
				mem.WithFile("foo.sha256", "b9e68e1bea3e5b19ca6b2f98b73a54b73daafaa250484902e09982e07a12e733  foo\n", 0644),
			),
			expect: "text\n",
		}, {
			description: "deeper path",
			filename:    "cat/foo",
			start: mem.New(
				mem.WithDir("cat", 0755),
				mem.WithFile("cat/foo", "text\n", 0644),
				mem.WithFile("cat/foo.sha256", "b9e68e1bea3e5b19ca6b2f98b73a54b73daafaa250484902e09982e07a12e733  foo\n", 0644),
			),
			expect: "text\n",
		}, {
			description: "no file",
			filename:    "./missing",
			start:       mem.New(),
			expectErr:   fs.ErrNotExist,
		}, {
			description: "no sha256 file",
			filename:    "./foo",
			start: mem.New(
				mem.WithDir(".", 0755),
				mem.WithFile("./foo", "text\n", 0644),
			),
			expectErr: fs.ErrNotExist,
		}, {
			description: "invalid sha",
			filename:    "./foo",
			start: mem.New(
				mem.WithDir(".", 0755),
				mem.WithFile("./foo", "text\n", 0644),
				mem.WithFile("./foo.sha256", "0000000000000000000000000000000000000000000000000000000000000000  foo\n", 0644),
			),
			expectErr: xafs.ErrInvalidSHA,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			// fmt.Print(tc.start)

			var buf []byte

			err := xafs.Operate(tc.start, xafs.ReadFileWithSHA256(tc.filename, &buf))

			assert.ErrorIs(err, tc.expectErr)
			if tc.expectErr == nil {
				assert.Equal([]byte(tc.expect), buf)
			} else {
				assert.Empty(buf)
			}
		})
	}
}

func TestWriteileWithSHA256(t *testing.T) {
	tests := []struct {
		description string
		filename    string
		data        string
		perm        fs.FileMode
		opts        []xafs.Option
		start       *mem.FS
		expect      *mem.FS
		expectErr   error
	}{
		{
			description: "simple path",
			filename:    "./foo",
			data:        "text\n",
			perm:        0644,
			start:       mem.New(mem.WithDir(".", 0755)),
			expect: mem.New(
				mem.WithDir(".", 0755),
				mem.WithFile("./foo", "text\n", 0644),
				mem.WithFile("./foo.sha256", "b9e68e1bea3e5b19ca6b2f98b73a54b73daafaa250484902e09982e07a12e733  foo\n", 0644),
			),
		}, {
			description: "no ./ prefix",
			filename:    "foo",
			data:        "text\n",
			perm:        0644,
			start:       mem.New(mem.WithDir(".", 0755)),
			expect: mem.New(
				mem.WithDir(".", 0755),
				mem.WithFile("foo", "text\n", 0644),
				mem.WithFile("foo.sha256", "b9e68e1bea3e5b19ca6b2f98b73a54b73daafaa250484902e09982e07a12e733  foo\n", 0644)),
		}, {
			description: "deeper path",
			filename:    "cat/foo",
			data:        "text\n",
			perm:        0644,
			start:       mem.New(mem.WithDir("cat", 0755)),
			expect: mem.New(
				mem.WithDir("cat", 0755),
				mem.WithFile("cat/foo", "text\n", 0644),
				mem.WithFile("cat/foo.sha256", "b9e68e1bea3e5b19ca6b2f98b73a54b73daafaa250484902e09982e07a12e733  foo\n", 0644)),
		}, {
			description: "over files",
			filename:    "foo",
			data:        "text\n",
			perm:        0644,
			start: mem.New(
				mem.WithDir(".", 0755),
				mem.WithFile("foo", "some other text\n", 0644),
				mem.WithFile("foo.sha256", "0000000000000000000000000000000000000000000000000000000000000000  foo\n", 0644)),
			expect: mem.New(
				mem.WithDir(".", 0755),
				mem.WithFile("foo", "text\n", 0644),
				mem.WithFile("foo.sha256", "b9e68e1bea3e5b19ca6b2f98b73a54b73daafaa250484902e09982e07a12e733  foo\n", 0644)),
		}, {
			description: "unable to write to file",
			filename:    "foo",
			data:        "text\n",
			perm:        0644,
			start: mem.New(
				mem.WithDir(".", 0755),
				mem.WithError("foo", errUnknown)),
			expectErr: errUnknown,
			expect: mem.New(
				mem.WithDir(".", 0755),
				mem.WithError("foo", errUnknown)),
		}, {
			description: "unable to write to sha file",
			filename:    "foo",
			data:        "text\n",
			perm:        0644,
			start: mem.New(
				mem.WithDir(".", 0755),
				mem.WithError("foo.sha256", errUnknown),
			),
			expectErr: errUnknown,
			expect: mem.New(
				mem.WithDir(".", 0755),
				mem.WithFile("foo", "text\n", 0644),
				mem.WithError("foo.sha256", errUnknown)),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)

			// fmt.Print(tc.start)
			fs := tc.start

			err := xafs.Operate(fs, tc.opts...)
			require.NoError(err)

			err = xafs.Operate(fs, xafs.WriteFileWithSHA256(tc.filename, []byte(tc.data), tc.perm))

			assert.ErrorIs(err, tc.expectErr)
			assert.Equal(tc.expect, fs)
		})
	}
}
