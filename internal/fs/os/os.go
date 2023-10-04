// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package os

import (
	iofs "io/fs"
	"os"
	"path/filepath"

	xafs "github.com/xmidt-org/xmidt-agent/internal/fs"
)

// Provide the os implementation of the internal FS interface.
type fs struct {
	base string
}

// New creates a new fs struct with the specified base directory.  If the
// directory does not exist, it is created with 0755 permissions.
func New(base string) (*fs, error) {
	tmp := fs{}

	err := xafs.Operate(&tmp, xafs.WithDirs(base, 0755))
	if err != nil {
		return nil, err
	}

	return &fs{
		base: base,
	}, nil
}

// Ensure that the fs struct implements the internal and fs.FS interfaces.
var _ iofs.FS = (*fs)(nil)
var _ xafs.FS = (*fs)(nil)

func (f *fs) Open(name string) (iofs.File, error) {
	return os.Open(filepath.Join(f.base, name))
}

func (f *fs) Mkdir(path string, perm iofs.FileMode) error {
	return os.Mkdir(filepath.Join(f.base, path), perm)
}

func (f *fs) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(f.base, name))
}

func (f *fs) WriteFile(name string, data []byte, perm iofs.FileMode) error {
	return os.WriteFile(filepath.Join(f.base, name), data, perm)
}
