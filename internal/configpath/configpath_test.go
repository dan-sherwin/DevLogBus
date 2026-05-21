package configpath

import "testing"

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
