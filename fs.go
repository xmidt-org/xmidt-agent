// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package xmidt_agent

import (
	"errors"

	"github.com/xmidt-org/xmidt-agent/internal/fs"
	"github.com/xmidt-org/xmidt-agent/internal/fs/os"
	"go.uber.org/fx"
)

func fsProvide() fx.Option {
	return fx.Provide(
		fx.Annotate(
			func(s Storage) (fs.FS, fs.FS, error) {
				var tmp, durable fs.FS
				var err, errs error

				if s.Temporary != "" {
					tmp, err = os.New(s.Temporary)
					errs = errors.Join(errs, err)
				}
				if s.Durable != "" {
					durable, err = os.New(s.Durable)
					errs = errors.Join(errs, err)
				}
				return tmp, durable, errs
			},
			fx.ResultTags(`name:"temporary_fs"`, `name:"durable_fs"`),
		),
	)
}
