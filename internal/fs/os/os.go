// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package os

import (
	iofs "io/fs"
	"os"

	xafs "github.com/xmidt-org/xmidt-agent/internal/fs"
)

// Provide the os implementation of the internal FS interface.
type fs struct{}

func New() *fs {
	return &fs{}
}

// Ensure that the fs struct implements the internal and fs.FS interfaces.
var _ iofs.FS = (*fs)(nil)
var _ xafs.FS = (*fs)(nil)

func (*fs) Open(name string) (iofs.File, error) {
	return os.Open(name)
}

func (*fs) Mkdir(path string, perm iofs.FileMode) error {
	return os.Mkdir(path, perm)
}

func (*fs) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (*fs) WriteFile(name string, data []byte, perm iofs.FileMode) error {
	return os.WriteFile(name, data, perm)
}
