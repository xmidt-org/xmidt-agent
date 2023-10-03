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

// MakeDir is an option that ensures the specified directory exists with the
// specified permissions.
func MakeDir(dir string, perm fs.FileMode) Option {
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
			if err != nil {
				return err
			}

			if !stat.IsDir() {
				return ErrNotDirectory
			}

			mode := stat.Mode()
			if (mode & fs.ModePerm & perm) != (fs.ModePerm & perm) {
				return fs.ErrPermission
			}

			return nil
		})
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
