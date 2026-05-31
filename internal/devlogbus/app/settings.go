package app

import (
	"fmt"
	"strings"

	"github.com/dan-sherwin/devlogbus/pkg/client"
	"github.com/dan-sherwin/go-app-settings"
)

var (
	LoggingLevel        = "info"
	Endpoint            = client.DefaultEndpoint()
	legacyBrokerNetwork string
	legacyBrokerAddress string
)

func init() {
	app_settings.RegisterStringSetting("logging_level", "Logging level (debug|info|warn|error)", &LoggingLevel)
	app_settings.RegisterSetting(&app_settings.Setting{
		Name:        "endpoint",
		Description: "DevLogBus broker endpoint: Unix socket path, unix:/path.sock, tcp://host:port, or host:port",
		GetFunc:     func() string { return Endpoint },
		SetFunc:     setEndpoint,
	})
	registerLegacyEndpointSettings()
}

func setEndpoint(s string) error {
	if s == "" {
		return fmt.Errorf("endpoint cannot be empty")
	}
	resolved, err := client.ParseEndpoint(s)
	if err != nil {
		return err
	}
	Endpoint = resolved.String()
	return nil
}

func registerLegacyEndpointSettings() {
	app_settings.RegisterSetting(&app_settings.Setting{
		Name:        "socket_path",
		Description: "Deprecated alias for endpoint",
		Hidden:      true,
		GetFunc:     func() string { return Endpoint },
		SetFunc:     setEndpoint,
	})
	app_settings.RegisterSetting(&app_settings.Setting{
		Name:        "broker_network",
		Description: "Deprecated alias for endpoint network",
		Hidden:      true,
		GetFunc:     func() string { return legacyBrokerNetwork },
		SetFunc: func(s string) error {
			legacyBrokerNetwork = strings.TrimSpace(s)
			return syncLegacyEndpoint()
		},
	})
	app_settings.RegisterSetting(&app_settings.Setting{
		Name:        "broker_address",
		Description: "Deprecated alias for endpoint address",
		Hidden:      true,
		GetFunc:     func() string { return legacyBrokerAddress },
		SetFunc: func(s string) error {
			legacyBrokerAddress = strings.TrimSpace(s)
			return syncLegacyEndpoint()
		},
	})
}

func syncLegacyEndpoint() error {
	if legacyBrokerAddress == "" {
		return nil
	}
	switch strings.ToLower(legacyBrokerNetwork) {
	case "", client.NetworkTCP:
		return setEndpoint(legacyBrokerAddress)
	case client.NetworkUnix:
		return setEndpoint(legacyBrokerAddress)
	default:
		return fmt.Errorf("unsupported broker network %q", legacyBrokerNetwork)
	}
}
