// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

//go:build !coverage

package quic

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	quic "github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// might be easier to just create an http3 server in the UT
// no good way to break this up

type Redirector interface {
	GetUrl(ctx context.Context, inUrl *url.URL) (*url.URL, error)
}

type UrlRedirector struct {
	tlsConfig       *tls.Config
	quicConfig      quic.Config
	credDecorator   func(http.Header) error
	conveyDecorator func(http.Header) error
}

// Retrieve the url from the redirect server, if there is one.  Stop the redirect.
// Possibly temporary solution until we figure
// out how to retrieve the new connection in the client after a seamless redirect.
func (r *UrlRedirector) GetUrl(ctx context.Context, inUrl *url.URL) (*url.URL, error) {
	outUrl := inUrl

	client := &http.Client{
		Transport: &http3.Transport{
			TLSClientConfig: r.tlsConfig,
			QUICConfig: &quic.Config{
				KeepAlivePeriod: time.Second,
			},
		},

		Timeout: time.Second * 30, // REMOVE
	}

	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, inUrl.String(), http.NoBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/msgpack")
	if err = r.credDecorator(req.Header); err != nil {
		return nil, err
	}
	if err = r.conveyDecorator(req.Header); err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	_, _ = io.Copy(io.Discard, resp.Body)

	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		redirectedUrl, err := url.Parse(resp.Header.Get("Location"))
		if err != nil {
			return nil, err
		}
		outUrl = redirectedUrl
	} else if resp.StatusCode >= 400 {
		errString := fmt.Sprintf("redirectServer returned status %d", resp.StatusCode)
		return nil, fmt.Errorf("%s: %w", errString, ErrFromRedirectServer)
	}

	client.CloseIdleConnections()

	return outUrl, nil
}
