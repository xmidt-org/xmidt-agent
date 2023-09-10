// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package fs

import "io/fs"

// Look here for interface options to include when we need more functionality:
// https://github.com/hack-pad/hackpadfs/blob/main/fs.go

// FS is the interface for a filesystem used by the program.
type FS interface {
	fs.FS

	// Mkdir creates a directory with the specified permissions.  Should match os.Mkdir().
	Mkdir(path string, perm fs.FileMode) error

	// ReadFile reads the file and returns the contents.  Should match os.ReadFile().
	ReadFile(name string) ([]byte, error)

	// WriteFile writes the file with the specified permissions.  Should match os.WriteFile().
	WriteFile(name string, data []byte, perm fs.FileMode) error
}
