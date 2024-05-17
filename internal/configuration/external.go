// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/goschtalt/goschtalt"
	"github.com/goschtalt/goschtalt/pkg/meta"
)

var (
	ErrInvalidConfig = errors.New("invalid configuration")
)

// External represents an external configuration file that is used to
// populate a map of string values.
type External struct {
	// Required determines if the external configuration file is required.  If
	// the file is required and cannot be found, the program will exit with an
	// error.
	Required bool

	// File is the path to the external configuration file based on the root path.
	File string

	// As is the alternate file extension to use when decoding the file.  If empty,
	// the file extension is used (like normal).
	As string

	// Origin is the origin of the external configuration file.  This is used in
	// by the configuration system to express where the configuration came from.
	Origin string

	// Remap is an array of from/to mappings that are used to remap the keys in the
	// external configuration file to the keys in the internal configuration map.
	Remap []Remap

	// root is the root filesystem to use when resolving the external configuration file.
	// If root is nil, the root filesystem is '/'.  This is used for testing purposes.
	root fs.FS
}

// Remap represents a key remapping from the external configuration file to the
// internal configuration map.
type Remap struct {
	// From is the key in the external configuration file.
	From string

	// To is the key in the internal configuration map.
	To string

	// Optional determines if the key is optional.  If the key is optional and cannot
	// be found in the external configuration file, the key is not added to the internal
	// configuration map.  If the key is not optional and cannot be found, the program
	// will exit with an error.
	Optional bool
}

// resolve is the internal implementation of the Resolve method that can more
// easily be tested.
func (ext External) resolve() (goschtalt.ExpanderFunc, error) {
	root := ext.root
	if root == nil {
		root = os.DirFS("/")
	}

	// if the path contains a leading slash, remove it.
	file := strings.TrimPrefix(ext.File, "/")
	opt := goschtalt.AddFilesAs(root, ext.As, file)
	if ext.Required {
		opt = goschtalt.AddFileAs(root, ext.As, file)
	}

	gs, err := goschtalt.New(opt, goschtalt.AutoCompile(true))
	if err != nil {
		return nil, err
	}

	rv := make(map[string]string, len(ext.Remap))
	for _, item := range ext.Remap {
		var val string

		if item.From == "" || item.To == "" {
			if item.Optional {
				continue
			}
			return nil, fmt.Errorf("%w: External remap is invalid", ErrInvalidConfig)
		}

		err = gs.Unmarshal(item.From, &val)
		if err != nil {
			if item.Optional && errors.Is(err, meta.ErrNotFound) {
				continue
			}
			return nil, err
		}

		rv[item.To] = val
	}

	return goschtalt.ExpanderFunc(
		func(s string) (string, bool) {
			if val, ok := rv[s]; ok {
				return val, true
			}
			return "", false
		}), nil
}

// Apply applies the external configurations defined to the goschtalt
// configuration system.
func Apply(gs *goschtalt.Config, name string, required bool, opts ...goschtalt.ExpandOption) error {
	return apply(gs, name, required, nil, opts...)
}

// apply is the internal implementation of the Apply method that can more
// easily be tested.
func apply(gs *goschtalt.Config, name string, required bool, fs fs.FS, opts ...goschtalt.ExpandOption) error {
	optional := goschtalt.Optional()
	if required {
		optional = goschtalt.Required()
	}

	externals, err := goschtalt.Unmarshal[[]External](gs, name, optional)
	if err != nil {
		return err
	}

	additional := make([]goschtalt.Option, 0, len(externals))
	for _, external := range externals {
		external.root = fs
		fn, err := external.resolve()
		if err != nil {
			return err
		}

		origin := external.Origin
		if external.Origin == "" {
			origin = external.File
		}

		opts = append(opts, goschtalt.WithOrigin(origin))
		exp := goschtalt.Expand(fn, opts...)
		additional = append(additional, exp)
	}

	return gs.With(additional...)
}
