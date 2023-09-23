// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/xmidt-org/xmidt-agent/internal/jwtxt"
	"github.com/xmidt-org/xmidt-agent/internal/jwtxt/event"
)

func main() {
	url := os.Args[1]
	id := os.Args[2]
	pemFile := os.Args[3]

	pem, err := os.ReadFile(pemFile)
	if err != nil {
		panic(err)
	}

	instructions, err := jwtxt.New(
		jwtxt.BaseURL(url),
		jwtxt.DeviceID(id),
		jwtxt.Algorithms("EdDSA",
			"ES256", "ES384", "ES512",
			"PS256", "PS384", "PS512",
			"RS256", "RS384", "RS512"),
		jwtxt.WithPEMs(pem),
		jwtxt.Timeout(5*time.Second),
		jwtxt.WithFetchListener(event.FetchListenerFunc(func(fe event.Fetch) {
			fmt.Printf("           FQDN: %s\n", fe.FQDN)
			fmt.Printf("         Server: %s\n", fe.Server)
			fmt.Printf("          Found: %t\n", fe.Found)
			fmt.Printf("        Timeout: %t\n", fe.Timeout)
			fmt.Printf("PriorExpiration: %s\n", fe.PriorExpiration)
			fmt.Printf("     Expiration: %s\n", fe.Expiration)
			fmt.Printf("   TemporaryErr: %t\n", fe.TemporaryErr)
			fmt.Printf("       Endpoint: %s\n", fe.Endpoint)
			fmt.Printf("        Payload: %s\n", fe.Payload)
			if fe.Err != nil {
				fmt.Printf("            Err: %s\n", fe.Err)
			} else {
				fmt.Printf("            Err: nil\n")
			}
		})),
	)
	if err != nil {
		panic(err)
	}

	endpoint, err := instructions.Endpoint(context.Background())
	if err != nil {
		panic(err)
	}

	fmt.Printf("\n\nEndpoint: '%s'\n", endpoint)
}
