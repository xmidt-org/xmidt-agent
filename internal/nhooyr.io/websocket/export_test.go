// SPDX-FileCopyrightText: 2023 Anmol Sethi <hi@nhooyr.io>
// SPDX-License-Identifier: ISC

//go:build !js
// +build !js

package websocket

import (
	"net"

	"github.com/xmidt-org/xmidt-agent/internal/nhooyr.io/websocket/internal/util"
)

func (c *Conn) RecordBytesWritten() *int {
	var bytesWritten int
	c.bw.Reset(util.WriterFunc(func(p []byte) (int, error) {
		bytesWritten += len(p)
		return c.rwc.Write(p)
	}))
	return &bytesWritten
}

func (c *Conn) RecordBytesRead() *int {
	var bytesRead int
	c.br.Reset(util.ReaderFunc(func(p []byte) (int, error) {
		n, err := c.rwc.Read(p)
		bytesRead += n
		return n, err
	}))
	return &bytesRead
}

var ErrClosed = net.ErrClosed

var ExportedDial = dial
var SecWebSocketAccept = secWebSocketAccept
var SecWebSocketKey = secWebSocketKey
var VerifyServerResponse = verifyServerResponse

var CompressionModeOpts = CompressionMode.opts
