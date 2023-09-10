// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package mem

import (
	"errors"
	"io"
	iofs "io/fs"
	"time"
)

var (
	ErrIsDir = errors.New("is a directory")
)

// File is an in-memory implementation of the iofs.File interface.
type File struct {
	Bytes  []byte
	Perm   iofs.FileMode
	name   string
	isDir  bool
	offset int
	closed bool
}

var _ iofs.File = (*File)(nil)

// Implement the iofs.File interface.

func (f *File) Stat() (iofs.FileInfo, error) {
	return &fileInfo{
		name:    f.name,
		size:    int64(len(f.Bytes)),
		mode:    f.Perm,
		modTime: time.Time{},
		isDir:   false,
	}, nil
}

func (f *File) Read(b []byte) (int, error) {
	if f.isDir {
		return 0, &iofs.PathError{Op: "read", Path: f.name, Err: ErrIsDir}
	}

	if f.closed {
		return 0, io.ErrClosedPipe
	}

	if len(f.Bytes) <= f.offset {
		return 0, io.EOF
	}

	n := copy(b, f.Bytes[f.offset:])
	f.offset += n
	return n, nil
}

func (f *File) Close() error {
	f.closed = true
	return nil
}

// fileInfo is an in-memory implementation of the iofs.FileInfo interface.
type fileInfo struct {
	name    string
	size    int64
	mode    iofs.FileMode
	modTime time.Time
	isDir   bool
}

var _ iofs.FileInfo = (*fileInfo)(nil)

// Implement the iofs.FileInfo interface.

func (fi *fileInfo) Name() string {
	return fi.name
}

func (fi *fileInfo) Size() int64 {
	return fi.size
}

func (fi *fileInfo) Mode() iofs.FileMode {
	return fi.mode
}

func (fi *fileInfo) ModTime() time.Time {
	return fi.modTime
}

func (fi *fileInfo) IsDir() bool {
	return fi.isDir
}

func (fi *fileInfo) Sys() interface{} {
	return nil
}
