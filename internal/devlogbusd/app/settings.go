package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dan-sherwin/devlogbus/pkg/client"
	"github.com/dan-sherwin/go-app-settings"
)

var (
	LoggingLevel      = "debug"
	Endpoint          = client.DefaultSocketPath()
	TCPListenAddress  = ""
	HTTPListenAddress = "127.0.0.1:7423"
	MaxRecords        = 5000
	Echo              = true
)

func init() {
	app_settings.RegisterStringSetting("logging_level", "Logging level (debug|info|warn|error)", &LoggingLevel)
	app_settings.RegisterSetting(&app_settings.Setting{
		Name:        "endpoint",
		Description: "Primary broker endpoint: Unix socket path, unix:/path.sock, tcp://host:port, or host:port",
		GetFunc:     func() string { return Endpoint },
		SetFunc:     setEndpoint,
	})
	app_settings.RegisterSetting(&app_settings.Setting{
		Name:        "socket_path",
		Description: "Deprecated alias for endpoint",
		Hidden:      true,
		GetFunc:     func() string { return Endpoint },
		SetFunc:     setEndpoint,
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
		Name:        "tcp_listen_address",
		Description: "Additional TCP listen address for Go/CLI clients; empty disables the extra listener",
		GetFunc:     func() string { return TCPListenAddress },
		SetFunc:     setTCPListenAddress,
	})
	app_settings.RegisterSetting(&app_settings.Setting{
		Name:        "max_records",
		Description: "Number of records to retain per source in the in-memory replay ring",
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

func setEndpoint(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("endpoint cannot be empty")
	}
	endpoint, err := client.ParseEndpoint(s)
	if err != nil {
		return err
	}
	Endpoint = endpoint.String()
	return nil
}

func setTCPListenAddress(s string) error {
	s = strings.TrimSpace(s)
	if s != "" {
		endpoint, err := client.ParseEndpoint("tcp:" + s)
		if err != nil {
			return err
		}
		if endpoint.Network != client.NetworkTCP {
			return fmt.Errorf("tcp_listen_address must be host:port")
		}
		s = endpoint.Address
	}
	TCPListenAddress = s
	return nil
}
