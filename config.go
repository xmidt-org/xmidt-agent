// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/xmidt-org/sallust"

type Config struct {
	SpecialValue string
	Logger       sallust.Config
}
