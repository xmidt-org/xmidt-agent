// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package mem

import (
	iofs "io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFS_Mkdir(t *testing.T) {
	tests := []struct {
		description string
		fs          FS
		path        string
		perm        iofs.FileMode
		expect      map[string]iofs.FileMode
		expectedErr error
	}{
		{
			description: "simple rel path",
			path:        "foo",
			perm:        0755,
			expect: map[string]iofs.FileMode{
				"foo": iofs.FileMode(0755),
			},
		}, {
			description: "simple abs path",
			fs: FS{
				Dirs: map[string]iofs.FileMode{
					"/": iofs.FileMode(0755),
				},
			},
			path: "/foo",
			perm: 0755,
			expect: map[string]iofs.FileMode{
				"/foo": iofs.FileMode(0755),
			},
		}, {
			description: "add a longer path",
			fs: FS{
				Dirs: map[string]iofs.FileMode{
					"foo": iofs.FileMode(0755),
				},
			},
			path: "foo/bar",
			perm: 0755,
			expect: map[string]iofs.FileMode{
				"foo/bar": iofs.FileMode(0755),
				"foo":     iofs.FileMode(0755),
			},
		}, {
			description: "fail on a longer path",
			path:        "foo/bar",
			perm:        0755,
			expectedErr: iofs.ErrNotExist,
		}, {
			description: "fail on an empty path",
			perm:        0755,
			expectedErr: iofs.ErrInvalid,
		}, {
			description: "err path",
			fs: FS{
				Errs: map[string]error{
					"foo": iofs.ErrInvalid,
				},
			},
			path:        "foo",
			perm:        0755,
			expectedErr: iofs.ErrInvalid,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			err := tc.fs.Mkdir(tc.path, tc.perm)

			assert.ErrorIs(err, tc.expectedErr)
			for k, v := range tc.expect {
				assert.Equal(v, tc.fs.Dirs[k])
			}
		})
	}
}

func TestFS_MkdirAll(t *testing.T) {
	tests := []struct {
		description string
		fs          FS
		path        string
		perm        iofs.FileMode
		expect      map[string]iofs.FileMode
		expectedErr error
	}{
		{
			description: "simple rel path",
			path:        "foo",
			perm:        0755,
			expect: map[string]iofs.FileMode{
				"foo": iofs.FileMode(0755),
			},
		}, {
			description: "simple abs path",
			fs: FS{
				Dirs: map[string]iofs.FileMode{
					"/": iofs.FileMode(0755),
				},
			},
			path: "/foo",
			perm: 0755,
			expect: map[string]iofs.FileMode{
				"/foo": iofs.FileMode(0755),
			},
		}, {
			description: "simple longer path",
			fs:          FS{},
			path:        "foo/bar",
			perm:        0755,
			expect: map[string]iofs.FileMode{
				"foo":     iofs.FileMode(0755),
				"foo/bar": iofs.FileMode(0755),
			},
		}, {
			description: "err path",
			fs: FS{
				Errs: map[string]error{
					"foo": iofs.ErrInvalid,
				},
			},
			path:        "foo",
			perm:        0755,
			expectedErr: iofs.ErrInvalid,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			err := tc.fs.MkdirAll(tc.path, tc.perm)

			assert.ErrorIs(err, tc.expectedErr)
			for k, v := range tc.expect {
				assert.Equal(v, tc.fs.Dirs[k])
			}
		})
	}
}
func TestFS_ReadFile(t *testing.T) {
	tests := []struct {
		description string
		fs          FS
		filename    string
		expect      string
		expectedErr error
	}{
		{
			description: "simple rel path",
			fs: FS{
				Files: map[string]File{
					"foo.txt": {
						Bytes: []byte("foo file"),
						Perm:  0644,
					},
				},
			},
			filename: "foo.txt",
			expect:   "foo file",
		}, {
			description: "longer rel path",
			fs: FS{
				Files: map[string]File{
					"bar/foo.txt": {
						Bytes: []byte("foo file"),
						Perm:  0644,
					},
				},
				Dirs: map[string]iofs.FileMode{
					"bar": iofs.FileMode(0755),
				},
			},
			filename: "bar/foo.txt",
			expect:   "foo file",
		}, {
			description: "err path",
			fs: FS{
				Errs: map[string]error{
					"foo.txt": iofs.ErrInvalid,
				},
			},
			filename:    "foo.txt",
			expectedErr: iofs.ErrInvalid,
		}, {
			description: "no directory defined",
			fs: FS{
				Files: map[string]File{
					"foo/bar.txt": {
						Bytes: []byte("foo file"),
						Perm:  0644,
					},
				},
			},
			filename:    "foo/bar.txt",
			expectedErr: iofs.ErrNotExist,
		}, {
			description: "no directory defined",
			fs: FS{
				Files: map[string]File{
					"foo/bar.txt": {
						Bytes: []byte("foo file"),
						Perm:  0644,
					},
				},
				Dirs: map[string]iofs.FileMode{
					"foo": iofs.FileMode(0644),
				},
			},
			filename:    "foo/bar.txt",
			expectedErr: iofs.ErrPermission,
		}, {
			description: "file is not readable",
			fs: FS{
				Files: map[string]File{
					"foo.txt": {
						Bytes: []byte("foo file"),
						Perm:  0200,
					},
				},
			},
			filename:    "foo.txt",
			expectedErr: iofs.ErrPermission,
		}, {
			description: "file is not readable due to the directory",
			fs: FS{
				Files: map[string]File{
					"foo.txt": {
						Bytes: []byte("foo file"),
						Perm:  0644,
					},
				},
				Dirs: map[string]iofs.FileMode{
					".": iofs.FileMode(0111),
				},
			},
			filename:    "foo.txt",
			expectedErr: iofs.ErrPermission,
		}, {
			description: "file does not exist",
			filename:    "foo.txt",
			expectedErr: iofs.ErrNotExist,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			got, err := tc.fs.ReadFile(tc.filename)

			assert.Equal(tc.expect, string(got))
			assert.ErrorIs(err, tc.expectedErr)
		})
	}
}

func TestFS_WriteFile(t *testing.T) {
	tests := []struct {
		description string
		fs          FS
		filename    string
		data        string
		perm        iofs.FileMode
		expect      FS
		expectedErr error
	}{
		{
			description: "simple rel path",
			filename:    "foo.txt",
			data:        "foo file",
			perm:        0644,
			expect: FS{
				Files: map[string]File{
					"foo.txt": {
						Bytes: []byte("foo file"),
						Perm:  0644,
					},
				},
			},
		}, {
			description: "longer path",
			filename:    "foo/bar.txt",
			data:        "bar file",
			perm:        0644,
			fs: FS{
				Dirs: map[string]iofs.FileMode{
					"foo": iofs.FileMode(0755),
				},
			},
			expect: FS{
				Dirs: map[string]iofs.FileMode{
					"foo": iofs.FileMode(0755),
				},
				Files: map[string]File{
					"foo/bar.txt": {
						Bytes: []byte("bar file"),
						Perm:  0644,
					},
				},
			},
		}, {
			description: "missing path",
			filename:    "foo/bar.txt",
			data:        "bar file",
			perm:        0644,
			expectedErr: iofs.ErrNotExist,
		}, {
			description: "error path",
			filename:    "foo.txt",
			data:        "foo file",
			perm:        0644,
			fs: FS{
				Errs: map[string]error{
					"foo.txt": iofs.ErrInvalid,
				},
			},
			expect: FS{
				Errs: map[string]error{
					"foo.txt": iofs.ErrInvalid,
				},
			},
			expectedErr: iofs.ErrInvalid,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			err := tc.fs.WriteFile(tc.filename, []byte(tc.data), tc.perm)

			assert.ErrorIs(err, tc.expectedErr)
			assert.Equal(tc.expect, tc.fs)
		})
	}
}

func TestFS_Open(t *testing.T) {
	tests := []struct {
		description string
		fs          FS
		name        string
		expect      *File
		expectedErr error
	}{
		{
			description: "open a file that doesn't exist",
			name:        "foo.txt",
			expectedErr: iofs.ErrNotExist,
		}, {
			description: "get an error opening a file",
			name:        "foo.txt",
			fs: FS{
				Errs: map[string]error{
					"foo.txt": iofs.ErrInvalid,
				},
			},
			expectedErr: iofs.ErrInvalid,
		}, {
			description: "open a file that exists",
			name:        "foo.txt",
			fs: FS{
				Files: map[string]File{
					"foo.txt": {
						Bytes: []byte("foo file"),
						Perm:  0644,
					},
				},
			},
			expect: &File{
				Bytes: []byte("foo file"),
				Perm:  0644,
				name:  "foo.txt",
			},
		}, {
			description: "open a dir that exists",
			name:        "foo",
			fs: FS{
				Dirs: map[string]iofs.FileMode{
					"foo": iofs.FileMode(0755),
				},
			},
			expect: &File{
				Perm:  0755,
				name:  "foo",
				isDir: true,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			got, err := tc.fs.Open(tc.name)

			assert.ErrorIs(err, tc.expectedErr)

			if tc.expect == nil {
				assert.Nil(got)
			} else {
				require.NotNil(got)

				assert.Equal(tc.expect.Bytes, got.(*File).Bytes)
				assert.Equal(tc.expect.Perm, got.(*File).Perm)
				assert.Equal(tc.expect.name, got.(*File).name)
				assert.Equal(tc.expect.isDir, got.(*File).isDir)
			}
		})
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		description string
		opts        []Option
		want        FS
	}{
		{
			description: "an empty fs",
		}, {
			description: "with an error",
			opts: []Option{
				WithError("foo.txt", iofs.ErrInvalid),
			},
			want: FS{
				Errs: map[string]error{
					"foo.txt": iofs.ErrInvalid,
				},
			},
		}, {
			description: "with a file",
			opts: []Option{
				WithFile("foo.txt", "foo file"),
			},
			want: FS{
				Files: map[string]File{
					"foo.txt": {
						Bytes: []byte("foo file"),
						Perm:  0644,
					},
				},
				Dirs: map[string]iofs.FileMode{
					".": iofs.FileMode(0755),
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			fs := tc.want
			assert.Equal(&fs, New(tc.opts...))
		})
	}
}
