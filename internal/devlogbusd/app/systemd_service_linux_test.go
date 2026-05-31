//go:build linux

package app

import (
	"strings"
	"testing"
)

func TestSystemdUnitUsesForegroundService(t *testing.T) {
	unit := systemdUnit("/usr/local/bin/devlogbusd")
	if !strings.Contains(unit, "Type=simple\n") {
		t.Fatalf("unit does not declare Type=simple:\n%s", unit)
	}
	if !strings.Contains(unit, "ExecStart=/usr/local/bin/devlogbusd run\n") {
		t.Fatalf("unit does not use foreground devlogbusd run:\n%s", unit)
	}
	if strings.Contains(unit, "PIDFile=") || strings.Contains(unit, "/var/run") {
		t.Fatalf("unit contains legacy pid file settings:\n%s", unit)
	}
}

func TestSystemdExecArgQuotesSpecialPaths(t *testing.T) {
	got := systemdExecArg("/opt/Dev LogBus/devlogbusd")
	if got != `"/opt/Dev LogBus/devlogbusd"` {
		t.Fatalf("systemdExecArg = %q", got)
	}
}
