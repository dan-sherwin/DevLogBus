package app

import (
	"log/slog"
	"os"

	"github.com/dan-sherwin/devlogbus/internal/configpath"
	"github.com/dan-sherwin/devlogbus/internal/devlogbus/app/consts"
	"github.com/dan-sherwin/go-app-settings"
)

func Setup() {
	initLogger()
	mergeSettingsVars()

	if !skipSettingsSetup(os.Args[1:]) {
		settingsFile, err := configpath.SettingsFile(consts.APPNAME)
		if err != nil {
			slog.Error("failed to resolve settings path", slog.String("error", err.Error()))
			os.Exit(1)
		}
		if err := app_settings.Setup(settingsFile, app_settings.SettingsOptions{}); err != nil {
			slog.Error("failed to setup settings", slog.String("error", err.Error()))
			os.Exit(1)
		}
		mergeSettingsVars()
	}

	processCLI()
	LoggingLevel = cliConfig.Logging.Level
	initLogger()
}

func mergeSettingsVars() {
	for key, value := range app_settings.SettingsVars() {
		vars[key] = value
	}
}

func skipSettingsSetup(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-h", "--help", "--help-long", "--help-short", "version", "buildinfo", "autoCompletions":
			return true
		}
	}
	return false
}
