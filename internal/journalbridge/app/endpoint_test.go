package app

import (
	"testing"

	"github.com/dan-sherwin/devlogbus/pkg/client"
)

func TestParseBrokerEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		network    string
		address    string
		socketPath string
	}{
		{
			name:       "absolute unix path",
			input:      "/tmp/devlogbus/devlogbus.sock",
			network:    client.NetworkUnix,
			address:    "/tmp/devlogbus/devlogbus.sock",
			socketPath: "/tmp/devlogbus/devlogbus.sock",
		},
		{
			name:       "unix scheme",
			input:      "unix:/tmp/devlogbus/devlogbus.sock",
			network:    client.NetworkUnix,
			address:    "/tmp/devlogbus/devlogbus.sock",
			socketPath: "/tmp/devlogbus/devlogbus.sock",
		},
		{
			name:    "tcp scheme",
			input:   "tcp://127.0.0.1:7422",
			network: client.NetworkTCP,
			address: "127.0.0.1:7422",
		},
		{
			name:    "tcp address",
			input:   "prod-box:7422",
			network: client.NetworkTCP,
			address: "prod-box:7422",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network, address, socketPath, err := parseBrokerEndpoint(tt.input)
			if err != nil {
				t.Fatalf("parse endpoint: %v", err)
			}
			if network != tt.network {
				t.Fatalf("network = %q, want %q", network, tt.network)
			}
			if address != tt.address {
				t.Fatalf("address = %q, want %q", address, tt.address)
			}
			if socketPath != tt.socketPath {
				t.Fatalf("socketPath = %q, want %q", socketPath, tt.socketPath)
			}
		})
	}
}

func TestParseBrokerEndpointRejectsTCPWithoutPort(t *testing.T) {
	if _, _, _, err := parseBrokerEndpoint("tcp://127.0.0.1"); err == nil {
		t.Fatal("expected tcp endpoint without port to fail")
	}
}
