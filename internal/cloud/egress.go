// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"github.com/xmidt-org/wrp-go/v3"
)

// Egress interface is the egress route used to handle wrp messages that
// targets something other than this device
type Egress interface {
	// HandleWrp is called whenever a message targets something other than this device (targets the cloud).
	HandleWrp(m wrp.Message) error
}
