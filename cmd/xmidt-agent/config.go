// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/tls"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/goschtalt/goschtalt"
	"github.com/xmidt-org/arrange/arrangehttp"
	"github.com/xmidt-org/arrange/arrangetls"
	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/sallust"
	"github.com/xmidt-org/wrp-go/v3"
	"go.uber.org/zap/zapcore"
	"gopkg.in/dealancer/validate.v2"
)

// Config is the configuration for the xmidt-agent.
type Config struct {
	Pubsub           Pubsub
	Websocket        Websocket
	Identity         Identity
	OperationalState OperationalState
	XmidtCredentials XmidtCredentials
	XmidtService     XmidtService
	Logger           sallust.Config
	Storage          Storage
	MockTr181        MockTr181
}

type Pubsub struct {
	// PublishTimeout sets the timeout for publishing a message
	PublishTimeout time.Duration
}

type Websocket struct {
	// Disable determines whether or not to disable xmidt-agent's websocket
	Disable bool
	// URLPath is the device registration url path
	URLPath string
	// BackUpURL is the back up XMiDT service endpoint in case `XmidtCredentials.URL` fails.
	BackUpURL string
	// AdditionalHeaders are any additional headers for the WS connection.
	AdditionalHeaders http.Header
	// FetchURLTimeout is the timeout for the fetching the WS url. If this is not set, the default is 30 seconds.
	FetchURLTimeout time.Duration
	// PingInterval is the ping interval allowed for the WS connection.
	PingInterval time.Duration
	// PingTimeout is the ping timeout for the WS connection.
	PingTimeout time.Duration
	// ConnectTimeout is the connect timeout for the WS connection.
	ConnectTimeout time.Duration
	// KeepAliveInterval is the keep alive interval for the WS connection.
	KeepAliveInterval time.Duration
	// IdleConnTimeout is the idle connection timeout for the WS connection.
	IdleConnTimeout time.Duration
	// TLSHandshakeTimeout is the TLS handshake timeout for the WS connection.
	TLSHandshakeTimeout time.Duration
	// ExpectContinueTimeout is the expect continue timeout for the WS connection.
	ExpectContinueTimeout time.Duration
	// MaxMessageBytes is the largest allowable message to send or receive.
	MaxMessageBytes int64
	// (optional) DisableV4 determines whether or not to allow IPv4 for the WS connection.
	// If this is not set, the default is false (IPv4 is enabled).
	// Either V4 or V6 can be disabled, but not both.
	DisableV4 bool
	// (optional) DisableV6 determines whether or not to allow IPv6 for the WS connection.
	// If this is not set, the default is false (IPv6 is enabled).
	// Either V4 or V6 can be disabled, but not both.
	DisableV6 bool
	// RetryPolicy sets the retry policy factory used for delaying between retry attempts for reconnection.
	RetryPolicy retry.Config
	// Once sets whether or not to only attempt to connect once.
	Once bool
}

// Identity contains the information that identifies the device.
type Identity struct {
	// DeviceID is the unique identifier for the device.  Generally this is a
	// MAC address of the "primary" network interface.
	DeviceID wrp.DeviceID

	// SerialNumber is the serial number of the device.
	SerialNumber string

	// Manufacturer is the name of the manufacturer of the device.
	HardwareModel string

	// HardwareManufacturer is the name of the manufacturer of the hardware.
	HardwareManufacturer string

	// FirmwareVersion is the version of the firmware.
	FirmwareVersion string

	// PartnerID is the identifier for the partner that the device is associated
	PartnerID string
}

// OperationalState contains the information about the device's operational state.
type OperationalState struct {
	// LastRebootReason is the reason for the last reboot.
	LastRebootReason string

	// BootTime is the time the device was last booted.
	BootTime time.Time
}

// XmidtCredentials contains the information needed to retrieve the credentials
// from the XMiDT credential server.
type XmidtCredentials struct {
	// URL is the URL of the XMiDT credential server.
	URL string

	// HTTPClient is the configuration for the HTTP client used to retrieve the
	// credentials.
	HTTPClient arrangehttp.ClientConfig

	// RefetchPercent is the percentage of the time between the last fetch and
	// the expiration time to refetch the credentials.  For example, if the
	// credentials are valid for 1 hour and the refetch percent is 90, then the
	// credentials will be refetched after 54 minutes.
	RefetchPercent float64

	// FileName is the name and path of the file to store the credentials.  There
	// will be another file with the same name and a ".sha256" extension that
	// contains the SHA256 hash of the credentials file.
	FileName string

	// FilePermissions is the permissions to use when creating the credentials
	// file.
	FilePermissions fs.FileMode
}

// XmidtService contains the configuration for the XMiDT service endpoint.
type XmidtService struct {
	// URL is the URL of the XMiDT service endpoint.  This is the endpoint that
	// the device will connect to or use as the fqdn for the JWT TXT redirector.
	URL string

	// Backoff is the parameters that limit the retry backoff algorithm.
	Backoff Backoff

	// JwtTxtRedirector is the configuration for the JWT TXT redirector.  If left
	// empty the JWT TXT redirector is disabled.
	JwtTxtRedirector JwtTxtRedirector
}

// JwtTxtRedirector contains the configuration for the JWT TXT redirector.
type JwtTxtRedirector struct {
	// AllowedAlgorithms is the list of allowed algorithms for the JWT TXT
	// redirector. Only specified algorithms will be used for verification.
	// Valid values are:
	//	- "EdDSA"
	//	- "ES256", "ES384", "ES512"
	//	- "PS256", "PS384", "PS512"
	//	- "RS256", "RS384", "RS512"
	AllowedAlgorithms []string

	// Timeout is the timeout for the JWT TXT redirector request.
	Timeout time.Duration

	// PEMs is the list of PEM-encoded public keys to use for verification.
	PEMs []string

	// PEMFiles is the list of files containing PEM-encoded public keys to use
	PEMFiles []string
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

type MockTr181 struct {
	FilePath string
	Enabled  bool
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
		HTTPClient: arrangehttp.ClientConfig{
			Timeout: 20 * time.Second,
			Transport: arrangehttp.TransportConfig{
				DisableKeepAlives: true,
				MaxIdleConns:      1,
			},
			TLS: &arrangetls.Config{
				MinVersion: tls.VersionTLS13,
			},
		},
	},
	XmidtService: XmidtService{
		Backoff: Backoff{
			MinDelay: 7 * time.Second,
			MaxDelay: 10 * time.Minute,
		},
		JwtTxtRedirector: JwtTxtRedirector{
			Timeout: 10 * time.Second,
			AllowedAlgorithms: []string{
				"EdDSA",
				"ES256", "ES384", "ES512",
				"PS256", "PS384", "PS512",
				"RS256", "RS384", "RS512",
			},
		},
	},
	Websocket: Websocket{
		URLPath:               "/api/v2/device",
		FetchURLTimeout:       30 * time.Second,
		PingInterval:          30 * time.Second,
		PingTimeout:           90 * time.Second,
		ConnectTimeout:        30 * time.Second,
		KeepAliveInterval:     30 * time.Second,
		IdleConnTimeout:       10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxMessageBytes:       256 * 1024,
		/*
			This retry policy gives us a very good approximation of the prior
			policy.  The important things about this policy are:

			1. The backoff increases up to the max.
			2. There is jitter that spreads the load so windows do not overlap.

			iteration | parodus   | this implementation
			----------+-----------+----------------
			0         | 0-1s      |   0.666 -  1.333
			1         | 1s-3s     |   1.333 -  2.666
			2         | 3s-7s     |   2.666 -  5.333
			3         | 7s-15s    |   5.333 -  10.666
			4         | 15s-31s   |  10.666 -  21.333
			5         | 31s-63s   |  21.333 -  42.666
			6         | 63s-127s  |  42.666 -  85.333
			7         | 127s-255s |  85.333 - 170.666
			8         | 255s-511s | 170.666 - 341.333
			9         | 255s-511s |           341.333
			n         | 255s-511s |           341.333
		*/
		RetryPolicy: retry.Config{
			Interval:    time.Second,
			Multiplier:  2.0,
			Jitter:      1.0 / 3.0,
			MaxInterval: 341*time.Second + 333*time.Millisecond,
		},
	},
	Pubsub: Pubsub{
		PublishTimeout: 200 * time.Millisecond,
	},
	Logger: sallust.Config{
		EncoderConfig: sallust.EncoderConfig{
			TimeKey:        "T",
			LevelKey:       "L",
			NameKey:        "N",
			CallerKey:      "C",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "M",
			StacktraceKey:  "S",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    "capital",
			EncodeTime:     "RFC3339Nano",
			EncodeDuration: "string",
			EncodeCaller:   "short",
		},
		Rotation: &sallust.Rotation{
			MaxSize:    1  // 1MB max/file
			MaxAge:     30,              // 30 days max
			MaxBackups: 10,              // max 10 files
		},
	},
	MockTr181: MockTr181{
		FilePath: "./mock_tr181.json",
	},
}
