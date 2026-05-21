package app

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/dan-sherwin/devlogbus/internal/devlogbusd/app/consts"
	"github.com/dan-sherwin/go-app-settings"
	"github.com/willabides/kongplete"
)

type CLIConfig struct {
	app_settings.SettingsDef
	Commands
	Run     RunCommand `cmd:"" default:"1" help:"Run the DevLogBus broker in the foreground"`
	Service ServiceDef `cmd:"" help:"Service management commands" name:"systemd"`
	Verbose bool       `short:"v" help:"Enable verbose output to stdout on platforms that otherwise log to the service manager"`
}

var (
	CLICommand *kong.Context
	cliConfig  CLIConfig
	vars       = kong.Vars{}
)

func processCLI() {
	parser := kong.Must(&cliConfig,
		kong.Name(consts.APPNAME),
		kong.Description("DevLogBus local structured log broker"),
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
