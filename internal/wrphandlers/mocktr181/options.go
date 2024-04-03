// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package mocktr181

import (
	"fmt"
)

// Sets the file location for the mocktr181 data
func FilePath(filePath string) Option {
	return optionFunc(
		func(h *Handler) error {
			if filePath == "" {
				return fmt.Errorf("%w: empty mock file location", ErrInvalidFileInput)
			}

			h.filePath = filePath

			return nil
		})
}

func Enabled(enabled bool) Option {
	return optionFunc(
		func(h *Handler) error {
			h.enabled = enabled
			return nil
		})
}
