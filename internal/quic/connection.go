// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

//go:build !coverage

package quic

import (
	"context"

	"github.com/quic-go/quic-go"
)

type Connection interface {
	// AcceptStream returns the next stream opened by the peer, blocking until one is available.
	// If the connection was closed due to a timeout, the error satisfies
	// the net.Error interface, and Timeout() will be true.
	AcceptStream(context.Context) (Stream, error)
	// AcceptUniStream returns the next unidirectional stream opened by the peer, blocking until one is available.
	// If the connection was closed due to a timeout, the error satisfies
	// the net.Error interface, and Timeout() will be true.
	//AcceptUniStream(context.Context) (Stream, error)
	// OpenStream opens a new bidirectional QUIC stream.
	// There is no signaling to the peer about new streams:
	// The peer can only accept the stream after data has been sent on the stream,
	// or the stream has been reset or closed.
	// When reaching the peer's stream limit, it is not possible to open a new stream until the
	// peer raises the stream limit. In that case, a StreamLimitReachedError is returned.
	OpenStream() (Stream, error)
	// OpenStreamSync opens a new bidirectional QUIC stream.
	// It blocks until a new stream can be opened.
	// There is no signaling to the peer about new streams:
	// The peer can only accept the stream after data has been sent on the stream,
	// or the stream has been reset or closed.
	//OpenStreamSync(context.Context) (Stream, error)
	// OpenUniStream opens a new outgoing unidirectional QUIC stream.
	// There is no signaling to the peer about new streams:
	// The peer can only accept the stream after data has been sent on the stream,
	// or the stream has been reset or closed.
	// When reaching the peer's stream limit, it is not possible to open a new stream until the
	// peer raises the stream limit. In that case, a StreamLimitReachedError is returned.
	//OpenUniStream() (Stream, error)
	// OpenUniStreamSync opens a new outgoing unidirectional QUIC stream.
	// It blocks until a new stream can be opened.
	// There is no signaling to the peer about new streams:
	// The peer can only accept the stream after data has been sent on the stream,
	// or the stream has been reset or closed.
	//OpenUniStreamSync(context.Context) (Stream, error)
	// LocalAddr returns the local address.
	//LocalAddr() net.Addr
	// RemoteAddr returns the address of the peer.
	//RemoteAddr() net.Addr
	// CloseWithError closes the connection with an error.
	// The error string will be sent to the peer.
	CloseWithError(quic.ApplicationErrorCode, string) error
	// Context returns a context that is canceled when the connection is closed.
	// The cancellation cause is set to the error that caused the connection to
	// close, or `context.Canceled` in case the listener is closed first.
	//Context() context.Context
	// ConnectionState returns basic details about the QUIC connection.
	// Warning: This API should not be considered stable and might change soon.
	//ConnectionState() quic.ConnectionState

	// SendDatagram sends a message using a QUIC datagram, as specified in RFC 9221.
	// There is no delivery guarantee for DATAGRAM frames, they are not retransmitted if lost.
	// The payload of the datagram needs to fit into a single QUIC packet.
	// In addition, a datagram may be dropped before being sent out if the available packet size suddenly decreases.
	// If the payload is too large to be sent at the current time, a DatagramTooLargeError is returned.
	//SendDatagram(payload []byte) error
	// ReceiveDatagram gets a message received in a datagram, as specified in RFC 9221.
	//ReceiveDatagram(context.Context) ([]byte, error)
}

type ConnectionWrapper struct {
	conn *quic.Conn
}

func (w ConnectionWrapper) AcceptStream(ctx context.Context) (Stream, error) {
	return w.conn.AcceptStream(ctx)
}

func (w ConnectionWrapper) OpenStream() (Stream, error) {
	return w.conn.OpenStream()
}

func (w ConnectionWrapper) CloseWithError(code quic.ApplicationErrorCode, msg string) error {
	return w.conn.CloseWithError(code, msg)
}
