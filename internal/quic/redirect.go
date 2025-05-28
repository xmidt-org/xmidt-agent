// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

//go:build !coverage

package quic

import (
	"bytes"
	"context"
	"crypto/tls"

	"fmt"

	"net/http"
	"net/url"

	"github.com/quic-go/quic-go"
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
		},
	}

	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	req, err := http.NewRequest(http.MethodPost, inUrl.String(), bytes.NewBuffer([]byte{}))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/msgpack")
	r.credDecorator(req.Header)
	r.conveyDecorator(req.Header)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("REMOVE redirect err %s", err.Error())
		return nil, err
	}
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

	return outUrl, nil
}
