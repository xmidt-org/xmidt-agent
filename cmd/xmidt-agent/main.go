// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/goschtalt/goschtalt"
	"github.com/xmidt-org/sallust"
	"github.com/xmidt-org/xmidt-agent/internal/adapters/libparodus"
	"github.com/xmidt-org/xmidt-agent/internal/credentials"
	"github.com/xmidt-org/xmidt-agent/internal/loglevel"
	"github.com/xmidt-org/xmidt-agent/internal/websocket"
	"github.com/xmidt-org/xmidt-agent/internal/wrphandlers/qos"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
)

const (
	applicationName = "xmidt-agent"
)

// These match what goreleaser provides.
var (
	commit  = "undefined"
	version = "undefined"
	date    = "undefined"
	builtBy = "undefined"
)

// CLI is the structure that is used to capture the command line arguments.
type CLI struct {
	Dev     bool     `optional:"" short:"d" help:"Run in development mode."`
	Show    bool     `optional:"" short:"s" help:"Show the configuration and exit."`
	Default string   `optional:""           help:"Output the default configuration file as the specified file."`
	Graph   string   `optional:"" short:"g" help:"Output the dependency graph to the specified file."`
	Files   []string `optional:"" short:"f" help:"Specific configuration files or directories."`
}

type LifeCycleIn struct {
	fx.In
	Logger           *zap.Logger
	LC               fx.Lifecycle
	Shutdowner       fx.Shutdowner
	WS               *websocket.Websocket
	LibParodus       *libparodus.Adapter
	QOS              *qos.Handler
	Cred             *credentials.Credentials
	WaitUntilFetched time.Duration `name:"wait_until_fetched"`
	Cancels          []func()      `group:"cancels"`
}

// xmidtAgent is the main entry point for the program.  It is responsible for
// setting up the dependency injection framework and returning the app object.
func xmidtAgent(args []string) (*fx.App, error) {
	app := fx.New(provideAppOptions(args))
	if err := app.Err(); err != nil {
		return nil, err
	}

	return app, nil
}

// provideAppOptions returns all fx options required to start the xmidt agent fx app.
func provideAppOptions(args []string) fx.Option {
	var (
		gscfg *goschtalt.Config

		// Capture the dependency tree in case we need to debug something.
		g fx.DotGraph

		// Capture the command line arguments.
		cli *CLI
	)

	opts := fx.Options(
		fx.Supply(cliArgs(args)),
		fx.Populate(&g),
		fx.Populate(&gscfg),
		fx.Populate(&cli),

		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),

		fx.Provide(
			provideCLI,
			provideLogger,
			provideConfig,
			provideCredentials,
			provideInstructions,
			provideWS,
			provideLibParodus,

			goschtalt.UnmarshalFunc[sallust.Config]("logger", goschtalt.Optional()),
			goschtalt.UnmarshalFunc[Identity]("identity"),
			goschtalt.UnmarshalFunc[OperationalState]("operational_state"),
			goschtalt.UnmarshalFunc[XmidtCredentials]("xmidt_credentials"),
			goschtalt.UnmarshalFunc[XmidtService]("xmidt_service"),
			goschtalt.UnmarshalFunc[Storage]("storage"),
			goschtalt.UnmarshalFunc[Websocket]("websocket"),
			goschtalt.UnmarshalFunc[MockTr181]("mock_tr_181"),
			goschtalt.UnmarshalFunc[Pubsub]("pubsub"),
			goschtalt.UnmarshalFunc[Metadata]("metadata"),
			goschtalt.UnmarshalFunc[NetworkService]("network_service"),
			goschtalt.UnmarshalFunc[QOS]("qos"),
			goschtalt.UnmarshalFunc[LibParodus]("lib_parodus"),

			provideNetworkService,
			provideMetadataProvider,
			loglevel.New,
		),

		fsProvide(),
		provideWRPHandlers(),

		fx.Invoke(
			lifeCycle,
		),
	)

	if cli != nil && cli.Graph != "" {
		_ = os.WriteFile(cli.Graph, []byte(g), 0600)
	}

	return opts
}

func main() {
	app, err := xmidtAgent(os.Args[1:])
	if err == nil {
		app.Run()
		return
	}

	fmt.Fprintln(os.Stderr, err)
	os.Exit(-1)
}

// Provides a named type so it's a bit easier to flow through & use in fx.
type cliArgs []string

// Handle the CLI processing and return the processed input.
func provideCLI(args cliArgs) (*CLI, error) {
	return provideCLIWithOpts(args, false)
}

func provideCLIWithOpts(args cliArgs, testOpts bool) (*CLI, error) {
	var cli CLI

	// Create a no-op option to satisfy the kong.New() call.
	var opt kong.Option = kong.OptionFunc(
		func(*kong.Kong) error {
			return nil
		},
	)

	if testOpts {
		opt = kong.Writers(nil, nil)
	}

	parser, err := kong.New(&cli,
		kong.Name(applicationName),
		kong.Description("The cpe agent for Xmidt service.\n"+
			fmt.Sprintf("\tVersion:  %s\n", version)+
			fmt.Sprintf("\tDate:     %s\n", date)+
			fmt.Sprintf("\tCommit:   %s\n", commit)+
			fmt.Sprintf("\tBuilt By: %s\n", builtBy)+
			"\n"+
			"Configuration files are read in the following order unless the "+
			"-f option is presented:\n"+
			"----------------------------------------------------\n"+
			"\t1.  $(pwd)/xmidt-agent.{properties|yaml|yml}\n"+
			"\t2.  $(pwd)/conf.d/*.{properties|yaml|yml}\n"+
			"\t3.  /etc/xmidt-agent/xmidt-agent.{properties|yaml|yml}\n"+
			"\t4.  /etc/xmidt-agent/xonfig.d/*.{properties|yaml|yml}\n"+
			"\nIf an exact file (1 or 3) is found, the search stops.  If "+
			"a conf.d directory is found, all files in that directory are "+
			"read in lexical order."+
			"\n\nWhen the -f is used, it adds to the list of files or directories "+
			"to read.  The -f option can be used multiple times."+
			"\n\nEnvironment variables are may be used in the configuration "+
			"files.  The environment variables are expanded in the "+
			"configuration files using the standard ${ VAR } syntax."+
			"\n\nIt is suggested to explore the configuration using the -s/--show "+
			"option to help ensure you understand how the client is configured."+
			"",
		),
		kong.UsageOnError(),
		opt,
	)
	if err != nil {
		return nil, err
	}

	if testOpts {
		parser.Exit = func(_ int) { panic("exit") }
	}

	_, err = parser.Parse(args)
	if err != nil {
		parser.FatalIfErrorf(err)
	}

	return &cli, nil
}

type LoggerIn struct {
	fx.In
	CLI *CLI
	Cfg sallust.Config
}

// Create the logger and configure it based on if the program is in
// debug mode or normal mode.
func provideLogger(in LoggerIn) (*zap.AtomicLevel, *zap.Logger, error) {
	if in.CLI.Dev {
		in.Cfg.EncoderConfig.EncodeLevel = "capitalColor"
		in.Cfg.EncoderConfig.EncodeTime = "RFC3339"
		in.Cfg.Level = "DEBUG"
		in.Cfg.Development = true
		in.Cfg.Encoding = "console"
		in.Cfg.OutputPaths = append(in.Cfg.OutputPaths, "stderr")
		in.Cfg.ErrorOutputPaths = append(in.Cfg.ErrorOutputPaths, "stderr")
	}

	zcfg, err := in.Cfg.NewZapConfig()
	if err != nil {
		return nil, nil, err
	}

	logger, err := in.Cfg.Build()

	return &zcfg.Level, logger, err
}

func onStart(cred *credentials.Credentials, ws *websocket.Websocket, libParodus *libparodus.Adapter, qos *qos.Handler, waitUntilFetched time.Duration, logger *zap.Logger) func(context.Context) error {
	logger = logger.Named("on_start")

	return func(ctx context.Context) (err error) {
		if err = ctx.Err(); err != nil {
			return err
		}

		if ws == nil {
			logger.Debug("websocket disabled")
			return err
		}

		// Allow operations where no credentials are desired (cred will be nil).
		if cred != nil {
			ctx, cancel := context.WithTimeout(ctx, waitUntilFetched)
			defer cancel()
			// blocks until an attempt to fetch the credentials has been made or the context is canceled
			cred.WaitUntilFetched(ctx)
		}

		ws.Start()
		err = libParodus.Start()
		qos.Start()

		return err
	}
}

func onStop(ws *websocket.Websocket, libParodus *libparodus.Adapter, qos *qos.Handler, shutdowner fx.Shutdowner, cancels []func(), logger *zap.Logger) func(context.Context) error {
	logger = logger.Named("on_stop")

	return func(context.Context) (err error) {
		if ws == nil {
			logger.Debug("websocket disabled")
			return nil
		}

		ws.Stop()
		libParodus.Stop()
		qos.Stop()
		for _, c := range cancels {
			if c == nil {
				continue
			}

			c()
		}

		return nil
	}
}

func lifeCycle(in LifeCycleIn) {
	logger := in.Logger.Named("fx_lifecycle")
	in.LC.Append(
		fx.Hook{
			OnStart: onStart(in.Cred, in.WS, in.LibParodus, in.QOS, in.WaitUntilFetched, logger),
			OnStop:  onStop(in.WS, in.LibParodus, in.QOS, in.Shutdowner, in.Cancels, logger),
		},
	)
}
