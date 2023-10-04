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

// Option is an interface for options that can be applied in order via the Operate function.
type Option interface {
	// Apply applies the option to the filesystem.
	Apply(FS) error
}

// OptionFunc is a function that implements the Option interface.
type OptionFunc func(FS) error

func (f OptionFunc) Apply(fs FS) error {
	return f(fs)
}

// Operate applies the specified options to the filesystem.  This allows for declaring
// a set of conditions that must be present (like creating a directory path) before
// the final operation is performed.
//
// Example:
//
//	Operate(fs,
//		MakeDir("tmp", 0755),
//		MakeDir("tmp/foo", 0755),
//		MakeDir("tmp/foo/bar", 0755),
//		WriteFile("tmp/foo/bar/baz.txt", []byte("hello world"), 0644))
func Operate(f FS, opts ...Option) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt.Apply(f); err != nil {
			return err
		}
	}
	return nil
}
