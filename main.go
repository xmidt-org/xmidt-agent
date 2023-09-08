// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/goschtalt/goschtalt"
	_ "github.com/goschtalt/goschtalt/pkg/typical"
	_ "github.com/goschtalt/yaml-decoder"
	_ "github.com/goschtalt/yaml-encoder"
	"github.com/xmidt-org/sallust"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
	Dev   bool     `optional:"" short:"d" help:"Run in development mode."`
	Show  bool     `optional:"" short:"s" help:"Show the configuration and exit."`
	Files []string `optional:"" short:"f" help:"Specific configuration files or directories."`
}

// xmidiAgent is the main entry point for the program.  It is responsible for
// setting up the dependency injection framework and invoking the program.
func xmidtAgent(args []string) error {
	var (
		gscfg *goschtalt.Config

		// Capture if the program is being run in dev mode so the extra stuff
		// is output as requested.
		dev devMode

		// Capture if the program should gracefully exit early & without
		// reporting an error via logging.
		early earlyExit
	)

	app := fx.New(
		fx.Supply(&early),
		fx.Supply(&dev),

		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),

		fx.Provide(
			// Handle the CLI processing and return the processed input.
			func(dev *devMode) (*CLI, error) {
				var cli CLI
				parser, err := kong.New(&cli,
					kong.Name(applicationName),
					kong.Description("The Xmidt agent.\n"+
						fmt.Sprintf("\tVersion:  %s\n", version)+
						fmt.Sprintf("\tDate:     %s\n", date)+
						fmt.Sprintf("\tCommit:   %s\n", commit)+
						fmt.Sprintf("\tBuilt By: %s\n", builtBy),
					),
					kong.UsageOnError(),
				)
				if err != nil {
					return nil, err
				}

				_, err = parser.Parse(args)
				parser.FatalIfErrorf(err)

				// Mark the devMode state so the collector can be output
				*dev = devMode(cli.Dev)
				return &cli, err
			},

			// Collect and process the configuration files and env vars and
			// produce a configuration object.
			func(cli *CLI) (*goschtalt.Config, error) {
				var err error
				gscfg, err = goschtalt.New(
					goschtalt.StdCfgLayout(applicationName, cli.Files...),
					goschtalt.ConfigIs("two_words"),

					// Seed the program with the default, built-in configuration
					goschtalt.AddValue("built-in", goschtalt.Root,
						Config{
							SpecialValue: "default",
						},
						goschtalt.AsDefault(), // Mark this as a default so it is ordered correctly
					),
				)

				return gscfg, err
			},

			goschtalt.UnmarshalFunc[sallust.Config]("logger", goschtalt.Optional()),

			// Create the logger and configure it based on if the program is in
			// debug mode or normal mode.
			func(cli *CLI, cfg sallust.Config) (*zap.Logger, error) {
				if cli.Dev {
					cfg.Level = "DEBUG"
					cfg.Development = true
					cfg.Encoding = "console"
					cfg.EncoderConfig = sallust.EncoderConfig{
						TimeKey:        "T",
						LevelKey:       "L",
						NameKey:        "N",
						CallerKey:      "C",
						FunctionKey:    zapcore.OmitKey,
						MessageKey:     "M",
						StacktraceKey:  "S",
						LineEnding:     zapcore.DefaultLineEnding,
						EncodeLevel:    "capitalColor",
						EncodeTime:     "RFC3339",
						EncodeDuration: "string",
						EncodeCaller:   "short",
					}
					cfg.OutputPaths = []string{"stderr"}
					cfg.ErrorOutputPaths = []string{"stderr"}
				}
				return cfg.Build()
			},
		),

		fx.Invoke(
			handleCLIShow,
		),
	)

	if dev {
		defer func() {
			fmt.Fprintln(os.Stderr, gscfg.Explain().String())
		}()
	}

	if err := app.Err(); err != nil || early {
		return err
	}

	app.Run()

	return nil
}

func main() {
	err := xmidtAgent(os.Args[1:])

	if err == nil {
		return
	}

	fmt.Fprintln(os.Stderr, err)
	os.Exit(-1)
}

// Provides a named type so it's a bit easier to flow through & use in fx.
type earlyExit bool

// Provides a named type so it's a bit easier to flow through & use in fx.
type devMode bool

// handleCLIShow handles the -s/--show option where the configuration is shown,
// then the program is exited.
func handleCLIShow(cli *CLI, cfg *goschtalt.Config, early *earlyExit) {
	if !cli.Show {
		return
	}

	fmt.Fprintln(os.Stdout, cfg.Explain().String())

	out, err := cfg.Marshal()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	} else {
		fmt.Fprintln(os.Stdout, "## Final Configuration\n---\n"+string(out))
	}

	*early = earlyExit(true)
}
