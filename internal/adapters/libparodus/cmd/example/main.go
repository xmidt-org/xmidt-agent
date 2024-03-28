// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"time"

	"github.com/alecthomas/kong"
	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/adapters/libparodus"
	"github.com/xmidt-org/xmidt-agent/internal/pubsub"

	// register transports
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

type CLI struct {
	URL  string `long:"url" description:"URL of the Parodus service" default:"tcp://127.0.0.1:6666"`
	Self string `long:"self" description:"Device ID" default:"mac:112233445566"`
}

func main() {
	var cli CLI
	_ = kong.Parse(&cli,
		kong.Name("example"),
		kong.Description("Example of using the libparodus package."),
		kong.UsageOnError(),
	)

	self, err := wrp.ParseDeviceID(cli.Self)
	must(err)

	ps, err := pubsub.New(self)
	must(err)

	lp, err := libparodus.New(cli.URL, ps)
	must(err)

	err = lp.Start()
	must(err)

	for {
		time.Sleep(5 * time.Second)
		fmt.Println(ps)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
