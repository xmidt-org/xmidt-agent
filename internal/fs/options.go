// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

var (
	// ErrNotDirectory is returned when a directory is expected but a file is found.
	ErrNotDirectory = errors.New("not a directory")
	ErrInvalidSHA   = errors.New("invalid SHA for file")
)

// Options provides a way to group multiple options together.
func Options(opts ...Option) Option {
	return OptionFunc(
		func(f FS) error {
			return Operate(f, opts...)
		})
}

// WithDir is an option that ensures the specified directory exists.  If it
// does not, create it with the specified permissions.
func WithDir(dir string, perm fs.FileMode) Option {
	return OptionFunc(
		func(f FS) error {
			file, err := f.Open(dir)
			if err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					return err
				}
				return f.Mkdir(dir, perm)
			}
			defer file.Close()

			stat, err := file.Stat()
			if err == nil {
				if !stat.IsDir() {
					return ErrNotDirectory
				}
			}

			return err
		})
}

// WithDirs is an option that ensures the specified directory path exists with
// the specified permissions.  The path is split on the path separator and
// each directory is created in order if needed.
//
// Notes:
//   - The path should not contain the filename or that will be created as a directory.
//   - The same permissions are applied to all directories that are created.
func WithDirs(path string, perm fs.FileMode) Option {
	dirs := strings.Split(path, string(filepath.Separator))
	if filepath.IsAbs(path) {
		dirs[0] = string(filepath.Separator)
	}

	var full string
	opts := make([]Option, 0, len(dirs))
	for _, dir := range dirs {
		full = filepath.Join(full, dir)
		opts = append(opts, WithDir(full, perm))
	}
	return Options(opts...)
}

// WithPath is an option that ensures the set of directories for the specified
// file exists.  The directory is determined by calling filepath.Dir on the name.
//
// Notes:
//   - The name should contain the filename and any path to ensure is present.
//   - The same permissions are applied to all directories that are created.
func WithPath(name string, perm fs.FileMode) Option {
	return WithDirs(filepath.Dir(name), perm)
}

// WriteFileWithSHA256 calculates and writes both the file and a checksum file.
//
// The format of the checksum file matches shasum format.  The name of the file
// is the file name with an `.sha256` appended to the file name.
func WriteFileWithSHA256(name string, data []byte, perm fs.FileMode) Option {
	return OptionFunc(
		func(f FS) error {
			if err := f.WriteFile(name, data, perm); err != nil {
				return err
			}

			sum := fmt.Sprintf("%x  %s\n", sha256.Sum256(data), filepath.Base(name))
			return f.WriteFile(shaSumName(name), []byte(sum), perm)
		})
}

// ReadFileWithSHA256 reads the specified file ensures the checksum matches.  The
// checksum is stored in a file of the same name ending with `.sha256`.  If the
// file checksum does not match an error is returned.
//
// The format of the .sha256 file is expected to match shasum format.
func ReadFileWithSHA256(name string, data *[]byte) Option {
	return OptionFunc(
		func(f FS) error {
			contents, err := f.ReadFile(name)
			if err != nil {
				return fmt.Errorf("%w: missing file: '%s'", err, name)
			}

			sName := shaSumName(name)
			sha, err := f.ReadFile(sName)
			if err != nil {
				return fmt.Errorf("%w: missing sha256 file: '%s'", err, sName)
			}
			want := strings.TrimSpace(string(sha))

			calc := fmt.Sprintf("%x  %s", sha256.Sum256(contents), filepath.Base(name))

			if calc != want {
				return fmt.Errorf("%w: expected sha: '%s', calculated: '%s'",
					ErrInvalidSHA, want, calc)
			}

			*data = contents

			return nil
		})
}

func shaSumName(name string) string {
	return name + ".sha256"
}
