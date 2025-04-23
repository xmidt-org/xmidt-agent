// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

//go:build !coverage

package quic

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

type Dialer interface {
	DialQuic(ctx context.Context, url *url.URL) (quic.Connection, error)
}

type QuicDialer struct {
	tlsConfig       *tls.Config
	quicConfig      quic.Config
	credDecorator   func(http.Header) error // TODO - these may need to be inline
	conveyDecorator func(http.Header) error
}

// NOTE - when using an http.Client, the quic connection seems to always
// get re-created and the client no longer had access to the current quic connection.  The below
// "dialer" uses the http3.ClientConn api directly and that api uses the passed in connection.
func (qd *QuicDialer) DialQuic(ctx context.Context, url *url.URL) (quic.Connection, error) {

	conn, err := quic.DialAddr(ctx, url.Host, qd.tlsConfig, &qd.quicConfig)
	if err != nil {
		// TODO - concerned there could be other reasons negotiatedProtocol is not populated
		if conn != nil && conn.ConnectionState().TLS.NegotiatedProtocol != "h3" {
			return nil, ErrHttp3NotSupported
		}
		return nil, err
	}

	roundTripper := &http3.Transport{
		TLSClientConfig: qd.tlsConfig,
		QUICConfig:      &qd.quicConfig,
	}

	h3Conn := roundTripper.NewClientConn(conn)

	reqStream, err := h3Conn.OpenRequestStream(ctx)
	if err != nil {

		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewBuffer([]byte{}))
	if err != nil {
		roundTripper.Close()
		return nil, err
	}

	req.Header.Set("Content-Type", "application/msgpack")
	qd.credDecorator(req.Header)
	qd.conveyDecorator(req.Header)

	err = reqStream.SendRequestHeader(req)
	if err != nil {
		return nil, err
	}

	resp, err := reqStream.ReadResponse()
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(io.Discard, resp.Body)
	if (err != nil) && errors.Is(err, io.EOF) {
		err = nil
	}

	resp.Body.Close()

	return h3Conn.Connection, err
}
