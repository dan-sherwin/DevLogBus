package app

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/dan-sherwin/devlogbus/internal/journalbridge/app/consts"
)

type (
	CLIConfig struct {
		Logging LoggingConfig `embed:"" prefix:"logging." group:"logging"`
		Run     RunCommand    `cmd:"" default:"1" help:"Stream journald records into DevLogBus"`
		BuildInfoCommandDef
	}
	LoggingConfig struct {
		Level string `default:"info" help:"debug, info, warn, error"`
	}
)

var (
	CLICommand *kong.Context
	cliConfig  CLIConfig
)

func processCLI() {
	parser := kong.Must(&cliConfig,
		kong.Name(consts.APPNAME),
		kong.Description("Bridge systemd-journald records into DevLogBus"),
		kong.ShortUsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
			Summary: true,
		}),
	)

	var err error
	CLICommand, err = parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)
}
