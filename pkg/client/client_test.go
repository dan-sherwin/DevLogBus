package client

import "testing"

func TestDefaultSocketPathIsStable(t *testing.T) {
	if got := DefaultSocketPath(); got != "/tmp/devlogbus/devlogbus.sock" {
		t.Fatalf("DefaultSocketPath = %q, want stable /tmp path", got)
	}
}

func TestNewWithOptionsDefaultsToUnixSocket(t *testing.T) {
	c := NewWithOptions(Options{})

	network, address, err := c.endpoint()
	if err != nil {
		t.Fatalf("endpoint returned error: %v", err)
	}
	if network != NetworkUnix {
		t.Fatalf("network = %q, want %q", network, NetworkUnix)
	}
	if address != DefaultSocketPath() {
		t.Fatalf("address = %q, want default socket path %q", address, DefaultSocketPath())
	}
}

func TestNewWithOptionsUsesTCPAddress(t *testing.T) {
	c := NewWithOptions(Options{Network: NetworkTCP, Address: "127.0.0.1:7422"})

	network, address, err := c.endpoint()
	if err != nil {
		t.Fatalf("endpoint returned error: %v", err)
	}
	if network != NetworkTCP {
		t.Fatalf("network = %q, want %q", network, NetworkTCP)
	}
	if address != "127.0.0.1:7422" {
		t.Fatalf("address = %q, want tcp address", address)
	}
}

func TestNewWithOptionsInfersTCPWhenAddressIsSet(t *testing.T) {
	c := NewWithOptions(Options{Address: "devbox:7422"})

	network, address, err := c.endpoint()
	if err != nil {
		t.Fatalf("endpoint returned error: %v", err)
	}
	if network != NetworkTCP {
		t.Fatalf("network = %q, want %q", network, NetworkTCP)
	}
	if address != "devbox:7422" {
		t.Fatalf("address = %q, want tcp address", address)
	}
}

func TestNewWithOptionsUsesEndpointSocketPath(t *testing.T) {
	c := NewWithOptions(Options{Endpoint: "/tmp/devlogbus/devlogbus.sock"})

	network, address, err := c.endpoint()
	if err != nil {
		t.Fatalf("endpoint returned error: %v", err)
	}
	if network != NetworkUnix {
		t.Fatalf("network = %q, want %q", network, NetworkUnix)
	}
	if address != "/tmp/devlogbus/devlogbus.sock" {
		t.Fatalf("address = %q, want socket path", address)
	}
	if c.Endpoint != "/tmp/devlogbus/devlogbus.sock" {
		t.Fatalf("client endpoint = %q, want socket path", c.Endpoint)
	}
}

func TestNewWithOptionsUsesEndpointTCPAddress(t *testing.T) {
	c := NewWithOptions(Options{Endpoint: "devbox:7422"})

	network, address, err := c.endpoint()
	if err != nil {
		t.Fatalf("endpoint returned error: %v", err)
	}
	if network != NetworkTCP {
		t.Fatalf("network = %q, want %q", network, NetworkTCP)
	}
	if address != "devbox:7422" {
		t.Fatalf("address = %q, want tcp address", address)
	}
	if c.Endpoint != "devbox:7422" {
		t.Fatalf("client endpoint = %q, want tcp address", c.Endpoint)
	}
}

func TestParseEndpointSupportsSchemes(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		network string
		address string
	}{
		{name: "unix scheme", raw: "unix:/tmp/devlogbus/devlogbus.sock", network: NetworkUnix, address: "/tmp/devlogbus/devlogbus.sock"},
		{name: "tcp scheme", raw: "tcp://127.0.0.1:7422", network: NetworkTCP, address: "127.0.0.1:7422"},
		{name: "tcp prefix", raw: "tcp:devbox:7422", network: NetworkTCP, address: "devbox:7422"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint, err := ParseEndpoint(tt.raw)
			if err != nil {
				t.Fatalf("parse endpoint: %v", err)
			}
			if endpoint.Network != tt.network {
				t.Fatalf("network = %q, want %q", endpoint.Network, tt.network)
			}
			if endpoint.Address != tt.address {
				t.Fatalf("address = %q, want %q", endpoint.Address, tt.address)
			}
		})
	}
}

func TestParseEndpointRejectsTCPWithoutPort(t *testing.T) {
	if _, err := ParseEndpoint("tcp://127.0.0.1"); err == nil {
		t.Fatal("expected tcp endpoint without port to fail")
	}
}
