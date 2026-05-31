package client

import (
	"path/filepath"
	"testing"
)

func TestDefaultSocketPathUsesConfiguredDirAndName(t *testing.T) {
	want := filepath.Join(DefaultSocketDir, DefaultSocketName)
	if got := DefaultSocketPath(); got != want {
		t.Fatalf("DefaultSocketPath = %q, want %q", got, want)
	}
}

func TestDefaultEndpointByPlatform(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{goos: "darwin", want: DefaultSocketPath()},
		{goos: "linux", want: DefaultSocketPath()},
		{goos: "windows", want: DefaultTCPAddress},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			if got := defaultEndpoint(tt.goos); got != tt.want {
				t.Fatalf("defaultEndpoint(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}

func TestNewWithOptionsDefaultsToPlatformEndpoint(t *testing.T) {
	c := NewWithOptions(Options{})

	network, address, err := c.endpoint()
	if err != nil {
		t.Fatalf("endpoint returned error: %v", err)
	}
	want, err := ParseEndpoint(DefaultEndpoint())
	if err != nil {
		t.Fatalf("parse default endpoint: %v", err)
	}
	if network != want.Network {
		t.Fatalf("network = %q, want %q", network, want.Network)
	}
	if address != want.Address {
		t.Fatalf("address = %q, want %q", address, want.Address)
	}
}

func TestNewWithOptionsUsesExplicitUnixDefault(t *testing.T) {
	c := NewWithOptions(Options{Network: NetworkUnix})

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
	socketPath := filepath.Clean("/tmp/devlogbus/devlogbus.sock")
	c := NewWithOptions(Options{Endpoint: socketPath})

	network, address, err := c.endpoint()
	if err != nil {
		t.Fatalf("endpoint returned error: %v", err)
	}
	if network != NetworkUnix {
		t.Fatalf("network = %q, want %q", network, NetworkUnix)
	}
	if address != socketPath {
		t.Fatalf("address = %q, want socket path", address)
	}
	if c.Endpoint != socketPath {
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
		{name: "unix scheme", raw: "unix:/tmp/devlogbus/devlogbus.sock", network: NetworkUnix, address: filepath.Clean("/tmp/devlogbus/devlogbus.sock")},
		{name: "tcp scheme", raw: "tcp://127.0.0.1:7422", network: NetworkTCP, address: "127.0.0.1:7422"},
		{name: "tcp prefix", raw: "tcp:devbox:7422", network: NetworkTCP, address: "devbox:7422"},
		{name: "single letter tcp host", raw: "x:7422", network: NetworkTCP, address: "x:7422"},
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

func TestParseEndpointTreatsPathLikeValuesAsUnix(t *testing.T) {
	tests := []string{
		"/tmp/devlogbus/devlogbus.sock",
		`\WINDOWS\SystemTemp\devlogbus-test.sock`,
		`C:\WINDOWS\SystemTemp\devlogbus-test.sock`,
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			endpoint, err := ParseEndpoint(raw)
			if err != nil {
				t.Fatalf("parse endpoint: %v", err)
			}
			if endpoint.Network != NetworkUnix {
				t.Fatalf("network = %q, want %q", endpoint.Network, NetworkUnix)
			}
			if endpoint.Address != filepath.Clean(raw) {
				t.Fatalf("address = %q, want %q", endpoint.Address, filepath.Clean(raw))
			}
		})
	}
}
