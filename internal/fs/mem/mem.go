// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package mem

import (
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"strings"

	xafs "github.com/xmidt-org/xmidt-agent/internal/fs"
)

const (
	separator = "/"
)

// Provide the in-memory implementation of the internal FS interface.
//
// This is a simple in-memory filesystem that can be used for testing.
// It is not thread-safe.
// There are two ways to use this:
//  1. Use the New() function to create a new FS instance and then use the
//     WithFile() and WithError() options to populate the filesystem.
//  2. Create a FS instance and then populate the Files, Dirs, and Errs
//     fields directly.
type FS struct {
	Files map[string]File
	Dirs  map[string]iofs.FileMode
	Errs  map[string]error
}

// Ensure that the fs struct implements the internal and fs.FS interfaces.
var _ iofs.FS = (*FS)(nil)
var _ xafs.FS = (*FS)(nil)

// New creates a new MemFS instance with the specified options.
func New(opts ...Option) *FS {
	var rv FS

	for _, opt := range opts {
		opt(&rv)
	}

	return &rv
}

// Option is a functional option for creating a MemFS instance.
type Option func(*FS)

// WithFile adds a file to the MemFS instance.  Only the first permission
// specified is used.  Errors are panic'd.
func WithFile(name, data string, perm iofs.FileMode) Option {
	return func(fs *FS) {
		err := fs.WriteFile(name, []byte(data), perm)
		if err != nil {
			panic(err)
		}
	}
}

// WithError adds an error to the MemFS instance.  When the file is opened,
// the error will be returned.
func WithError(name string, err error) Option {
	return func(fs *FS) {
		if fs.Errs == nil {
			fs.Errs = make(map[string]error)
		}
		fs.Errs[name] = err
	}
}

// WithDir adds a directory to the MemFS instance.
func WithDir(name string, perm iofs.FileMode) Option {
	return func(fs *FS) {
		err := fs.MkdirAll(name, perm)
		if err != nil {
			panic(err)
		}
	}
}

// Implement the iofs.FS interface.

func (fs *FS) Open(name string) (iofs.File, error) {
	if err := fs.hasPerms(name, iofs.FileMode(0222)); err != nil {
		return nil, err
	}

	if f, found := fs.Files[name]; found {
		f.name = name
		return &f, nil
	}

	if dir, found := fs.Dirs[name]; found {
		return &File{
			Perm:  dir,
			name:  name,
			isDir: true,
		}, nil
	}

	return nil, fmt.Errorf("%w: file named: '%s'", iofs.ErrNotExist, name)
}

func (fs *FS) Mkdir(path string, perm iofs.FileMode) error {
	if err := fs.hasPerms(path, iofs.FileMode(0111)); err != nil {
		return err
	}

	if fs.Dirs == nil {
		fs.Dirs = make(map[string]iofs.FileMode)
	}

	// Don't change the permissions if the directory is already there.
	if _, found := fs.Dirs[path]; !found {
		fs.Dirs[path] = perm
	}
	return nil
}

func (fs *FS) MkdirAll(path string, perm iofs.FileMode) error {
	var abs bool
	if strings.HasPrefix(path, separator) {
		path = strings.TrimPrefix(path, separator)
		abs = true
	}

	dirs := strings.Split(path, separator)

	for i := range dirs {
		full := strings.Join(dirs[:i+1], separator)
		if abs {
			full = separator + full
		}
		if err := fs.Mkdir(full, perm); err != nil {
			return err
		}
	}

	return nil
}

func (fs *FS) ReadFile(name string) ([]byte, error) {
	if err := fs.hasPerms(name, iofs.FileMode(0444)); err != nil {
		return nil, err
	}

	if f, found := fs.Files[name]; found {
		return f.Bytes, nil
	}
	return nil, fmt.Errorf("%w: file named: '%s'", iofs.ErrNotExist, name)
}

func (fs *FS) WriteFile(name string, data []byte, perm iofs.FileMode) error {
	if err := fs.hasPerms(name, iofs.FileMode(0222)); err != nil {
		return err
	}

	if fs.Files == nil {
		fs.Files = make(map[string]File)
	}
	fs.Files[name] = File{Bytes: data, Perm: perm}

	return nil
}

func (fs *FS) hasPerms(name string, perm iofs.FileMode) error {
	if name == "" {
		return iofs.ErrInvalid
	}

	if err, found := fs.Errs[name]; found {
		return err
	}

	path := filepath.Dir(name)
	dir, found := fs.Dirs[path]
	if !found {
		if path != "." {
			return iofs.ErrNotExist
		}
		// Default to 0755 for the root directory.
		dir = 0755
	}

	if dir&iofs.FileMode(0111) == 0 {
		return fmt.Errorf("%w: dir named: '%s'", iofs.ErrPermission, path)
	}

	if dir&perm == 0 {
		return fmt.Errorf("%w: dir named: '%s'", iofs.ErrPermission, path)
	}

	if f, found := fs.Files[name]; found {
		if f.Perm&perm == 0 {
			return fmt.Errorf("%w: file named: '%s'", iofs.ErrPermission, name)
		}
	}

	return nil
}

// Remove this in favor of pp ... except I can't download pp right now.
/*
func (fs *FS) String() string {
	buf := strings.Builder{}

	buf.WriteString("Files:\n")
	for k, v := range fs.Files {
		fmt.Fprintf(&buf, "  '%s': '%s'\n", k, string(v.Bytes))
	}
	buf.WriteString("Dirs:\n")
	for k, v := range fs.Dirs {
		fmt.Fprintf(&buf, "  '%s': %v\n", k, v)
	}
	buf.WriteString("Errs:\n")
	for k, v := range fs.Errs {
		fmt.Fprintf(&buf, "  '%s': %v\n", k, v)
	}

	return buf.String()
}
*/
