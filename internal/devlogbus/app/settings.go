package app

import (
	"fmt"

	"github.com/dan-sherwin/devlogbus/pkg/client"
	"github.com/dan-sherwin/go-app-settings"
)

var (
	LoggingLevel = "info"
	SocketPath   = client.DefaultSocketPath()
)

func init() {
	app_settings.RegisterStringSetting("logging_level", "Logging level (debug|info|warn|error)", &LoggingLevel)
	app_settings.RegisterSetting(&app_settings.Setting{
		Name:        "socket_path",
		Description: "Unix socket path for the DevLogBus broker",
		GetFunc:     func() string { return SocketPath },
		SetFunc: func(s string) error {
			if s == "" {
				return fmt.Errorf("socket_path cannot be empty")
			}
			SocketPath = s
			return nil
		},
	})
}
