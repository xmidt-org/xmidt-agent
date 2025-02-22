// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/websocket"
	"github.com/xmidt-org/xmidt-agent/internal/websocket/event"
)

// CLI is the structure that is used to capture the command line arguments.
type CLI struct {
	Id   string `optional:"" default:"mac:112233445566"                         help:"The id of the device."`
	URL  string `optional:"" default:"https://fabric.example.com/api/v2/device" help:"The URL for the WS connection."`
	V4   bool   `optional:"" short:"4" name:"4" xor:"ipmode"                    help:"Only use IPv4"`
	V6   bool   `optional:"" short:"6" name:"6" xor:"ipmode"                    help:"Only use IPv6"`
	Once bool   `optional:""                                                    help:"Only attempt to connect once."`
}

func main() {
	var cli CLI

	parser, err := kong.New(&cli,
		kong.Name("example"),
		kong.Description("The test agent for websocket service.\n"),
		kong.UsageOnError(),
	)
	if err != nil {
		panic(err)
	}

	_, err = parser.Parse(os.Args[1:])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	id, err := wrp.ParseDeviceID(cli.Id)
	if err != nil {
		panic(err)
	}

	opts := []websocket.Option{
		websocket.DeviceID(id),
		websocket.URL(cli.URL),
		websocket.AddConnectListener(
			event.ConnectListenerFunc(
				func(e event.Connect) {
					fmt.Println(e)
				})),
		websocket.AddDisconnectListener(
			event.DisconnectListenerFunc(
				func(e event.Disconnect) {
					fmt.Println(e)
				})),
	}

	if cli.V4 {
		opts = append(opts, websocket.WithIPv6(false))
	}

	if cli.V6 {
		opts = append(opts, websocket.WithIPv4(false))
	}

	if cli.Once {
		opts = append(opts, websocket.Once())
	}

	ws, err := websocket.New(opts...)
	if err != nil {
		panic(err)
	}

	ws.Start()
	defer ws.Stop()

	time.Sleep(time.Minute)
}
