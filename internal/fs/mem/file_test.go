// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package mem

import (
	"io"
	iofs "io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFile_Stat(t *testing.T) {
	tests := []struct {
		description string
		file        File
		want        iofs.FileInfo
		expectedErr error
	}{
		{
			description: "a simple file",
			file: File{
				Bytes: []byte("hello"),
				Perm:  iofs.FileMode(0644),
				name:  "foo",
			},
			want: &fileInfo{
				name:  "foo",
				size:  5,
				mode:  iofs.FileMode(0644),
				isDir: false,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			got, err := tc.file.Stat()

			assert.ErrorIs(err, tc.expectedErr)
			assert.Equal(tc.want.Name(), got.Name())
			assert.Equal(tc.want.Size(), got.Size())
			assert.Equal(tc.want.Mode(), got.Mode())
			assert.Equal(tc.want.ModTime(), got.ModTime())
			assert.Equal(tc.want.IsDir(), got.IsDir())
			assert.Equal(tc.want.Sys(), got.Sys())
		})
	}
}

func TestFile_Read(t *testing.T) {
	tests := []struct {
		description string
		file        File
		buf         []byte
		want        int
		before      func(file *File)
		expect      string
		expectedErr error
	}{
		{
			description: "a simple file read",
			file: File{
				Bytes: []byte("hello"),
				Perm:  iofs.FileMode(0644),
				name:  "foo",
			},
			buf:    make([]byte, 5),
			want:   5,
			expect: "hello",
		}, {
			description: "a simple file read that is shorter than the buffer",
			file: File{
				Bytes: []byte("hello"),
				Perm:  iofs.FileMode(0644),
				name:  "foo",
			},
			buf:    make([]byte, 1),
			want:   1,
			expect: "h",
		}, {
			description: "read an empty file",
			file: File{
				Perm: iofs.FileMode(0644),
				name: "foo",
			},
			buf:         make([]byte, 5),
			want:        0,
			expectedErr: io.EOF,
		}, {
			description: "read a directory",
			file: File{
				Perm:  iofs.FileMode(0755),
				name:  "foo",
				isDir: true,
			},
			buf:         make([]byte, 5),
			want:        0,
			expectedErr: ErrIsDir,
		}, {
			description: "read a closed file",
			file: File{
				Perm: iofs.FileMode(0644),
				name: "foo",
			},
			before: func(file *File) {
				file.Close()
			},
			buf:         make([]byte, 5),
			want:        0,
			expectedErr: io.ErrClosedPipe,
		}, {
			description: "read a partially read file",
			file: File{
				Bytes: []byte("hello"),
				Perm:  iofs.FileMode(0644),
				name:  "foo",
			},
			before: func(file *File) {
				buf := make([]byte, 1)
				_, _ = file.Read(buf)
			},
			buf:    make([]byte, 5),
			want:   4,
			expect: "ello",
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			file := tc.file
			if tc.before != nil {
				tc.before(&file)
			}

			got, err := file.Read(tc.buf)

			assert.ErrorIs(err, tc.expectedErr)
			assert.Equal(tc.want, got)
			assert.Equal(tc.expect, string(tc.buf[:tc.want]))
		})
	}
}
