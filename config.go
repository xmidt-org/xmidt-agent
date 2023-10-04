// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/goschtalt/goschtalt"
	"github.com/xmidt-org/arrange/arrangehttp"
	"github.com/xmidt-org/sallust"
	"github.com/xmidt-org/wrp-go/v3"
	"gopkg.in/dealancer/validate.v2"
)

type Config struct {
	Identity         Identity
	OperationalState OperationalState
	XmidtCredentials XmidtCredentials
	XmidtService     XmidtService
	Logger           sallust.Config
	Storage          Storage
}

type Identity struct {
	DeviceID             wrp.DeviceID
	SerialNumber         string
	HardwareModel        string
	HardwareManufacturer string
	FirmwareVersion      string
	PartnerID            string
}

type OperationalState struct {
	LastRebootReason string
	BootTime         time.Time
}

type XmidtCredentials struct {
	URL             string
	HTTPClient      arrangehttp.ClientConfig
	RefetchPercent  float64
	FileName        string
	FilePermissions fs.FileMode
}

type XmidtService struct {
	URL              string
	JwtTxtRedirector JwtTxtRedirector
	Backoff          Backoff
}

type JwtTxtRedirector struct {
	Required          bool
	AllowedAlgorithms []string
	Timeout           time.Duration
	PEMs              []string
	PEMFiles          []string
}

// Backoff defines the parameters that limit the retry backoff algorithm.
// The retries are a geometric progression.
// 1, 3, 7, 15, 31 ... n = (2n+1)
type Backoff struct {
	MinDelay time.Duration
	MaxDelay time.Duration
}

type Storage struct {
	Temporary string
	Durable   string
}

// Collect and process the configuration files and env vars and
// produce a configuration object.
func provideConfig(cli *CLI) (*goschtalt.Config, error) {
	gs, err := goschtalt.New(
		goschtalt.StdCfgLayout(applicationName, cli.Files...),
		goschtalt.ConfigIs("two_words"),
		goschtalt.DefaultUnmarshalOptions(
			goschtalt.WithValidator(
				goschtalt.ValidatorFunc(validate.Validate),
			),
		),

		// Seed the program with the default, built-in configuration.
		// Mark this as a default so it is ordered correctly.
		goschtalt.AddValue("built-in", goschtalt.Root, defaultConfig,
			goschtalt.AsDefault()),
	)
	if err != nil {
		return nil, err
	}

	if cli.Show {
		// handleCLIShow handles the -s/--show option where the configuration is
		// shown, then the program is exited.
		//
		// Exit with success because if the configuration is broken it will be
		// very hard to debug where the problem originates.  This way you can
		// see the configuration and then run the service with the same
		// configuration to see the error.

		fmt.Fprintln(os.Stdout, gs.Explain().String())

		out, err := gs.Marshal()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			fmt.Fprintln(os.Stdout, "## Final Configuration\n---\n"+string(out))
		}

		os.Exit(0)
	}

	var tmp Config
	err = gs.Unmarshal(goschtalt.Root, &tmp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "There is a critical error in the configuration.")
		fmt.Fprintln(os.Stderr, "Run with -s/--show to see the configuration.")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		// Exit here to prevent a very difficult to debug error from occurring.
		os.Exit(0)
	}

	return gs, nil
}

// -----------------------------------------------------------------------------
// Keep the default configuration at the bottom of the file so it is easy to
// see what the default configuration is.
// -----------------------------------------------------------------------------

var defaultConfig = Config{
	XmidtCredentials: XmidtCredentials{
		RefetchPercent:  90.0,
		FileName:        "credentials.msgpack",
		FilePermissions: fs.FileMode(0600),
	},
	XmidtService: XmidtService{
		JwtTxtRedirector: JwtTxtRedirector{
			Required: true,
			Timeout:  10 * time.Second,
			AllowedAlgorithms: []string{
				"EdDSA",
				"ES256", "ES384", "ES512",
				"PS256", "PS384", "PS512",
				"RS256", "RS384", "RS512",
			},
		},
	},
}
