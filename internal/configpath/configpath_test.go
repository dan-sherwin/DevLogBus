package configpath

import (
	"errors"
	"testing"
)

func TestConfigDirUsesServicePathForRootEvenWhenUserConfigDirWorks(t *testing.T) {
	got, err := configDir("darwin", 0, func() (string, error) {
		return "/var/root/Library/Application Support", nil
	})
	if err != nil {
		t.Fatalf("configDir returned error: %v", err)
	}
	if got != "/Library/Application Support" {
		t.Fatalf("configDir = %q, want /Library/Application Support", got)
	}
}

func TestConfigDirUsesUserConfigDirForNonRoot(t *testing.T) {
	got, err := configDir("darwin", 501, func() (string, error) {
		return "/Users/dsherwin/Library/Application Support", nil
	})
	if err != nil {
		t.Fatalf("configDir returned error: %v", err)
	}
	if got != "/Users/dsherwin/Library/Application Support" {
		t.Fatalf("configDir = %q, want user config dir", got)
	}
}

func TestConfigDirReturnsUserConfigErrorForNonRoot(t *testing.T) {
	want := errors.New("no home")
	_, err := configDir("darwin", 501, func() (string, error) {
		return "", want
	})
	if !errors.Is(err, want) {
		t.Fatalf("configDir error = %v, want %v", err, want)
	}
}

func TestServiceConfigDirUsesMacLaunchDaemonPath(t *testing.T) {
	got, ok := serviceConfigDir("darwin", 0)
	if !ok {
		t.Fatalf("serviceConfigDir returned ok=false, want true")
	}
	if got != "/Library/Application Support" {
		t.Fatalf("serviceConfigDir = %q, want /Library/Application Support", got)
	}
}

func TestServiceConfigDirRejectsNonRoot(t *testing.T) {
	if got, ok := serviceConfigDir("darwin", 501); ok {
		t.Fatalf("serviceConfigDir = %q, true; want no fallback for non-root", got)
	}
}
