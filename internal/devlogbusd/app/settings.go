package app

import (
	"fmt"
	"strconv"

	"github.com/dan-sherwin/devlogbus/pkg/client"
	"github.com/dan-sherwin/go-app-settings"
)

var (
	LoggingLevel      = "debug"
	SocketPath        = client.DefaultSocketPath()
	HTTPListenAddress = "127.0.0.1:7423"
	MaxRecords        = 5000
	Echo              = true
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
	app_settings.RegisterSetting(&app_settings.Setting{
		Name:        "http_listen_address",
		Description: "HTTP listen address for browser clients; empty disables HTTP",
		GetFunc:     func() string { return HTTPListenAddress },
		SetFunc: func(s string) error {
			HTTPListenAddress = s
			return nil
		},
	})
	app_settings.RegisterSetting(&app_settings.Setting{
		Name:        "max_records",
		Description: "Number of records to retain in the in-memory replay ring",
		GetFunc:     func() string { return strconv.Itoa(MaxRecords) },
		SetFunc: func(s string) error {
			v, err := strconv.Atoi(s)
			if err != nil {
				return err
			}
			if v <= 0 {
				return fmt.Errorf("max_records must be greater than zero")
			}
			MaxRecords = v
			return nil
		},
	})
	app_settings.RegisterBoolSetting("echo", "Print received records to stdout", &Echo)
}
