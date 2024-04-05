// SPDX-FileCopyrightText: 2023 Anmol Sethi <hi@nhooyr.io>
// SPDX-License-Identifier: ISC

package websocket

import "net"

func (nc *netConn) RemoteAddr() net.Addr {
	return websocketAddr{}
}

func (nc *netConn) LocalAddr() net.Addr {
	return websocketAddr{}
}
