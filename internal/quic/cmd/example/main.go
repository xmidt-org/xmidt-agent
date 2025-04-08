// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/quic-go/quic-go"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/event"
	myquic "github.com/xmidt-org/xmidt-agent/internal/quic"
)

// CLI is the structure that is used to capture the command line arguments.
type CLI struct {
	Id   string `optional:"" default:"mac:112233445566"                         help:"The id of the device."`
	URL  string `optional:"" default:"https://fabric.example.com/api/v2/device" help:"The URL for the WS connection."`
	V4   bool   `optional:"" short:"4" name:"4" xor:"ipmode"                    help:"Only use IPv4"`
	V6   bool   `optional:"" short:"6" name:"6" xor:"ipmode"                    help:"Only use IPv6"`
	Once bool   `optional:""                                                    help:"Only attempt to connect once."`
}

type MessageListenerFunc func(wrp.Message)

func (f MessageListenerFunc) OnMessage(m wrp.Message) {
	f(m)
}

// Run this and then run a server... otherwise I don't know what the point of this is because it runs in the same
// process as the quic client
func main() {
	var cli CLI

	parser, err := kong.New(&cli,
		kong.Name("example"),
		kong.Description("The test agent for quic service.\n"),
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

	opts := []myquic.Option{
		myquic.DeviceID(id),
		myquic.URL(cli.URL),
		myquic.HTTP3Client(
			&myquic.Http3ClientConfig{
				QuicConfig: quic.Config{},
				TlsConfig: tls.Config{},
			},
			
		),
		myquic.AddConnectListener(
			event.ConnectListenerFunc(
				func(e event.Connect) {
					fmt.Println(e)
				})),
		myquic.AddDisconnectListener(
			event.DisconnectListenerFunc(
				func(e event.Disconnect) {
					fmt.Println(e)
				})),
		myquic.AddMessageListener(
			MessageListenerFunc(
				func(m wrp.Message) {
					fmt.Println(m) // send a message back
				})),
	}

	if cli.Once {
		opts = append(opts, myquic.Once())
	}

	q, err := myquic.New(opts...)
	if err != nil {
		panic(err)
	}

	q.Start()
	defer q.Stop()

	time.Sleep(time.Minute)
}
