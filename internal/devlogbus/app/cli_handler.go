package app

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/dan-sherwin/devlogbus/internal/devlogbus/app/consts"
	"github.com/dan-sherwin/go-app-settings"
	"github.com/willabides/kongplete"
)

type CLIConfig struct {
	app_settings.SettingsDef
	Commands
}

var (
	CLICommand *kong.Context
	cliConfig  CLIConfig
	vars       = kong.Vars{}
)

func processCLI() {
	parser := kong.Must(&cliConfig,
		kong.Name(consts.APPNAME),
		kong.Description("DevLogBus CLI and TUI entrypoint"),
		kong.ShortUsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
			Summary: true,
		}),
		vars,
	)

	kongplete.Complete(parser)
	var err error
	CLICommand, err = parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)
}
