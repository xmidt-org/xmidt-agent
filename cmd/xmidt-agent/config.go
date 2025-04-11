// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	_ "embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/goschtalt/goschtalt"
	_ "github.com/goschtalt/goschtalt/pkg/typical"
	_ "github.com/goschtalt/properties-decoder"
	_ "github.com/goschtalt/yaml-decoder"
	_ "github.com/goschtalt/yaml-encoder"
	"github.com/xmidt-org/arrange/arrangehttp"
	"github.com/xmidt-org/retry"
	"github.com/xmidt-org/sallust"
	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/configuration"
	"github.com/xmidt-org/xmidt-agent/internal/net"
	myquic "github.com/xmidt-org/xmidt-agent/internal/quic"
	"github.com/xmidt-org/xmidt-agent/internal/wrphandlers/qos"
	"gopkg.in/dealancer/validate.v2"
)

//go:embed default-config.yaml
var defaultConfigFile []byte

// Config is the configuration for the xmidt-agent.
type Config struct {
	Pubsub           Pubsub
	Websocket        Websocket
	LibParodus       LibParodus
	Identity         Identity
	OperationalState OperationalState
	XmidtCredentials XmidtCredentials
	XmidtService     XmidtService
	Logger           sallust.Config
	Storage          Storage
	MockTr181        MockTr181
	QOS              QOS
	Externals        []configuration.External
	XmidtAgentCrud   XmidtAgentCrud
	Metadata         Metadata
	NetworkService   NetworkService
	Quic             Quic
}

type LibParodus struct {
	// ParodusServiceURL is the service url used by libparodus
	ParodusServiceURL string
	// KeepAliveInterval is the keep alive interval for libparodus.
	KeepAliveInterval time.Duration
	// ReceiveTimeout is the Receive timeout for libparodus.
	ReceiveTimeout time.Duration
	// SendTimeout is the send timeout for libparodus.
	SendTimeout time.Duration
}

type QOS struct {
	// MaxQueueBytes is the allowable max size of the qos' priority queue, based on the sum of all queued wrp message's payload.
	MaxQueueBytes int64
	// MaxMessageBytes is the largest allowable wrp message payload.
	MaxMessageBytes int
	// Priority determines what is used [newest, oldest message] for QualityOfService tie breakers and trimming,
	// with the default being to prioritize the newest messages.
	Priority qos.PriorityType
	// LowExpires determines when low qos messages are trimmed.
	LowExpires time.Duration
	// MediumExpires determines when medium qos messages are trimmed.
	MediumExpires time.Duration
	// HighExpires determines when high qos messages are trimmed.
	HighExpires time.Duration
	// CriticalExpires determines when critical qos messages are trimmed.
	CriticalExpires time.Duration
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
	// InactivityTimeout is the inactivity timeout for the WS connection.
	InactivityTimeout time.Duration
	// PingWriteTimeout is the ping timeout for the WS connection.
	PingWriteTimeout time.Duration
	// SendTimeout is the send timeout for the WS connection.
	SendTimeout time.Duration
	// HTTPClient is the configuration for the HTTP client.
	HTTPClient arrangehttp.ClientConfig
	// KeepAliveInterval is the keep alive interval for the WS connection.
	KeepAliveInterval time.Duration
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

type Quic struct {
	// Disable determines whether or not to disable xmidt-agent's quic client
	Disable bool
	// URLPath is the device registration url path
	URLPath string
	// BackUpURL is the back up XMiDT service endpoint in case `XmidtCredentials.URL` fails.
	BackUpURL string
	// SendTimeout is the send timeout for the WS connection.
	SendTimeout time.Duration
	// AdditionalHeaders are any additional headers for the WS connection.
	AdditionalHeaders http.Header
	// FetchURLTimeout is the timeout for the fetching the Quic url. If this is not set, the default is 30 seconds.
	FetchURLTimeout time.Duration
	// The client to used to connect to the redirect server
	// HttpClient arrangehttp.ClientConfig
	// // Config for quic connection
	QuicClient myquic.Http3ClientConfig
	// MaxMessageBytes is the largest allowable message to send or receive. (TODO
	MaxMessageBytes int64
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

	// network interface to use for connection from agent to webpa cloud
	WebpaInterfaceUsed string
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

	// WaitUntilFetched is the time the xmidt-agent blocks on startup until an attempt to fetch the credentials has been made.
	WaitUntilFetched time.Duration
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

type XmidtAgentCrud struct {
	ServiceName string
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
	FilePath    string
	Enabled     bool
	ServiceName string
}

type Metadata struct {
	Fields []string
}

type NetworkService struct {
	// list of allowed network interfaces to connect to xmidt in priority order, first is highest
	AllowedInterfaces map[string]net.AllowedInterface
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
		// Seed the program with the default, built-in configuration
		goschtalt.AddBuffer("!built-in.yaml", defaultConfigFile, goschtalt.AsDefault()),
	)
	if err != nil {
		return nil, err
	}

	// Externals are a list of individually processed external configuration
	// files.  Each external configuration file is processed and the resulting
	// map is used to populate the configuration.
	//
	// This is done after the initial configuration has been calculated because
	// the external configurations are listed in the configuration.
	if err = configuration.Apply(gs, "externals", false); err != nil {
		return nil, err
	}

	if cli.Default != "" {
		err := os.WriteFile("./"+cli.Default, defaultConfigFile, 0644) // nolint: gosec
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(-1)
		}
		os.Exit(0)
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
