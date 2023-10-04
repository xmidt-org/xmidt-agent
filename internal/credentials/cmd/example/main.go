// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/golang-jwt/jwt/v5"
	"github.com/xmidt-org/wrp-go/v3"
	cred "github.com/xmidt-org/xmidt-agent/internal/credentials"
	"github.com/xmidt-org/xmidt-agent/internal/credentials/event"
)

type CLI struct {
	URL         string        `long:"url" help:"URL of the credential service." required:"true"`
	ID          string        `long:"id" help:"Device ID." default:"mac:112233445566"`
	Private     string        `long:"private" help:"mTLS private key to use."`
	Public      string        `long:"public" help:"mTLS public key to use."`
	CA          string        `long:"ca" help:"mTLS CA to use."`
	Timeout     time.Duration `long:"timeout" help:"HTTP client timeout." default:"5s"`
	RedirectMax int           `long:"redirect-max" help:"Maximum number of redirects to follow." default:"10"`
}

func main() {
	var cli CLI
	_ = kong.Parse(&cli,
		kong.Name("example"),
		kong.Description("Example of using the credentials package."),
		kong.UsageOnError(),
	)

	client := http.DefaultClient

	if cli.Private != "" || cli.Public != "" {
		if cli.Private == "" || cli.Public == "" {
			panic("--private and --public must be specified together")
		}

		cert, err := tls.LoadX509KeyPair(cli.Public, cli.Private)
		if err != nil {
			panic(err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		if cli.CA != "" {
			caCert, err := os.ReadFile(cli.CA)
			if err != nil {
				panic(err)
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			tlsConfig.RootCAs = caCertPool
		}
		tr := &http.Transport{TLSClientConfig: tlsConfig}

		// Create an HTTP client with the custom transport
		client.Transport = tr
	}

	if cli.Timeout > 0 {
		client.Timeout = cli.Timeout
	}

	if cli.RedirectMax > 0 {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) > cli.RedirectMax {
				return fmt.Errorf("stopped after %d redirects", cli.RedirectMax)
			}
			return nil
		}
	}

	credentials, err := cred.New(
		cred.URL(cli.URL),
		cred.MacAddress(wrp.DeviceID(cli.ID)),
		cred.HTTPClient(client),
		cred.SerialNumber("1234567890"),
		cred.HardwareModel("model"),
		cred.HardwareManufacturer("manufacturer"),
		cred.FirmwareVersion("version"),
		cred.LastRebootReason("reason"),
		cred.XmidtProtocol("protocol"),
		cred.BootRetryWait(1),
		cred.AddFetchListener(
			event.FetchListenerFunc(func(fe event.Fetch) {
				fmt.Println("Fetch:")
				fmt.Printf("  At:         %s\n", fe.At.Format(time.RFC3339))
				fmt.Printf("  Duration:   %s\n", fe.Duration)
				fmt.Printf("  UUID:       %s\n", fe.UUID)
				fmt.Printf("  StatusCode: %d\n", fe.StatusCode)
				fmt.Printf("  RetryIn:    %s\n", fe.RetryIn)
				fmt.Printf("  Expiration: %s\n", fe.Expiration.Format(time.RFC3339))
				if fe.Err != nil {
					fmt.Printf("  Err:        %s\n", fe.Err)
				} else {
					fmt.Println("  Err:        nil")
				}
			}),
		),
	)
	if err != nil {
		panic(err)
	}

	credentials.Start()
	defer credentials.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	credentials.WaitUntilFetched(ctx)
	token, expires, err := credentials.Credentials()
	if err != nil {
		panic(err)
	}

	fmt.Printf("JWT:     %s\n", token)
	fmt.Printf("Expires: %s\n", expires.Format(time.RFC3339))

	claims := jwt.RegisteredClaims{}
	parser := jwt.NewParser()
	_, parts, err := parser.ParseUnverified(token, &claims)
	if err != nil {
		panic(err)
	}

	fmt.Println("Claims:")
	fmt.Printf("  ID:             %s\n", claims.ID)
	fmt.Printf("  ExpirationTime: %s\n", claims.ExpiresAt)
	fmt.Printf("  IssuedAt:       %s\n", claims.IssuedAt)
	fmt.Printf("  NotBefore:      %s\n", claims.NotBefore)
	fmt.Printf("  Issuer:         %s\n", claims.Issuer)
	fmt.Printf("  Subject:        %s\n", claims.Subject)
	fmt.Printf("  Audience:       %s\n", claims.Audience)

	header, err := parser.DecodeSegment(parts[0])
	if err != nil {
		panic(err)
	}

	body, err := parser.DecodeSegment(parts[1])
	if err != nil {
		panic(err)
	}

	fmt.Println("Parts:")
	fmt.Printf("  Header: %s\n", header)
	fmt.Printf("  Body:   %s\n", body)
}
