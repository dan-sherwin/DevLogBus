package app

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/dan-sherwin/devlogbus/pkg/client"
)

func brokerClient(endpoint string, network string, address string, socketPath string) (*client.Client, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return client.NewWithOptions(client.Options{
			Network:    network,
			Address:    address,
			SocketPath: socketPath,
		}), nil
	}

	network, address, socketPath, err := parseBrokerEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	return client.NewWithOptions(client.Options{
		Network:    network,
		Address:    address,
		SocketPath: socketPath,
	}), nil
}

func parseBrokerEndpoint(raw string) (network string, address string, socketPath string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", "", fmt.Errorf("devlogbus endpoint cannot be empty")
	}

	lower := strings.ToLower(raw)
	switch {
	case strings.HasPrefix(lower, "unix://"):
		socketPath = raw[len("unix://"):]
		if socketPath == "" {
			return "", "", "", fmt.Errorf("unix devlogbus endpoint requires a socket path")
		}
		socketPath = filepath.Clean(socketPath)
		return client.NetworkUnix, socketPath, socketPath, nil
	case strings.HasPrefix(lower, "unix:"):
		socketPath = raw[len("unix:"):]
		if socketPath == "" {
			return "", "", "", fmt.Errorf("unix devlogbus endpoint requires a socket path")
		}
		socketPath = filepath.Clean(socketPath)
		return client.NetworkUnix, socketPath, socketPath, nil
	case strings.HasPrefix(lower, "tcp://"):
		parsed, parseErr := url.Parse(raw)
		if parseErr != nil {
			return "", "", "", fmt.Errorf("parse tcp devlogbus endpoint: %w", parseErr)
		}
		if parsed.Host == "" {
			return "", "", "", fmt.Errorf("tcp devlogbus endpoint requires host:port")
		}
		if err := validateTCPAddress(parsed.Host); err != nil {
			return "", "", "", err
		}
		return client.NetworkTCP, parsed.Host, "", nil
	case strings.HasPrefix(lower, "tcp:"):
		address = raw[len("tcp:"):]
		if address == "" {
			return "", "", "", fmt.Errorf("tcp devlogbus endpoint requires host:port")
		}
		if err := validateTCPAddress(address); err != nil {
			return "", "", "", err
		}
		return client.NetworkTCP, address, "", nil
	}

	if err := validateTCPAddress(raw); err == nil {
		return client.NetworkTCP, raw, "", nil
	}
	socketPath = filepath.Clean(raw)
	return client.NetworkUnix, socketPath, socketPath, nil
}

func validateTCPAddress(address string) error {
	if _, _, err := net.SplitHostPort(address); err != nil {
		return fmt.Errorf("tcp devlogbus endpoint must be host:port: %w", err)
	}
	return nil
}
