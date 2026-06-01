package app

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dan-sherwin/devlogbus/internal/devlogbusd/app/consts"
)

type (
	ServiceDef struct {
		Install InstallServiceCommand `cmd:"" group:"Systemd" help:"Install the service"`
		Remove  RemoveServiceCommand  `cmd:"" group:"Systemd" help:"Remove the service"`
		Start   StartServiceCommand   `cmd:"" group:"Systemd" help:"Start the service"`
		Stop    StopServiceCommand    `cmd:"" group:"Systemd" help:"Stop the service"`
		Restart RestartServiceCommand `cmd:"" group:"Systemd" help:"Restart the service"`
		Status  ServiceStatusCommand  `cmd:"" group:"Systemd" help:"Show service status"`
	}
	InstallServiceCommand struct{}
	RemoveServiceCommand  struct{}
	StartServiceCommand   struct{}
	StopServiceCommand    struct{}
	RestartServiceCommand struct{}
	ServiceStatusCommand  struct{}
)

func setupSystemdService() {}

func (i *InstallServiceCommand) Run() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	if err := os.WriteFile(systemdUnitPath(), []byte(systemdUnit(executable)), 0o644); err != nil {
		slog.Error("service install failed", slog.String("error", err.Error()))
		return fmt.Errorf("write %s: %w", systemdUnitPath(), err)
	}
	if _, err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	if _, err := runSystemctl("enable", systemdUnitName()); err != nil {
		return err
	}
	fmt.Printf("Installed %s\n", systemdUnitName())
	return nil
}

func (r *RemoveServiceCommand) Run() error {
	if _, err := runSystemctl("disable", "--now", systemdUnitName()); err != nil {
		slog.Warn("service disable failed", slog.String("error", err.Error()))
	}
	if err := os.Remove(systemdUnitPath()); err != nil && !os.IsNotExist(err) {
		slog.Error("service remove failed", slog.String("error", err.Error()))
		return fmt.Errorf("remove %s: %w", systemdUnitPath(), err)
	}
	if _, err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	if _, err := runSystemctl("reset-failed", systemdUnitName()); err != nil {
		slog.Debug("service reset-failed skipped", slog.String("error", err.Error()))
	}
	fmt.Printf("Removed %s\n", systemdUnitName())
	return nil
}

func (s *StartServiceCommand) Run() error {
	if _, err := runSystemctl("start", systemdUnitName()); err != nil {
		slog.Error("service start failed", slog.String("error", err.Error()))
		return err
	}
	fmt.Printf("Started %s\n", systemdUnitName())
	return nil
}

func (s *StopServiceCommand) Run() error {
	if _, err := runSystemctl("stop", systemdUnitName()); err != nil {
		slog.Error("service stop failed", slog.String("error", err.Error()))
		return err
	}
	fmt.Printf("Stopped %s\n", systemdUnitName())
	return nil
}

func (r *RestartServiceCommand) Run() error {
	if _, err := runSystemctl("restart", systemdUnitName()); err != nil {
		slog.Error("service restart failed", slog.String("error", err.Error()))
		return err
	}
	fmt.Printf("Restarted %s\n", systemdUnitName())
	return nil
}

func (s *ServiceStatusCommand) Run() error {
	output, err := runSystemctl("status", systemdUnitName(), "--no-pager")
	fmt.Println(output)
	if err != nil {
		slog.Error("service status failed", slog.String("error", err.Error()))
		return err
	}
	return nil
}

func systemdUnitName() string {
	return consts.APPNAME + ".service"
}

func systemdUnitPath() string {
	return filepath.Join("/etc/systemd/system", systemdUnitName())
}

func systemdUnit(executable string) string {
	return fmt.Sprintf(`[Unit]
Description=DevLogBus real-time development log broker
After=network.target

[Service]
Type=simple
ExecStart=%s run
Restart=on-failure
RestartSec=2s

[Install]
WantedBy=multi-user.target
`, systemdExecArg(executable))
}

func systemdExecArg(value string) string {
	if !strings.ContainsAny(value, " \t'\"\\") {
		return value
	}
	return strconv.Quote(value)
}

func runSystemctl(args ...string) (string, error) {
	cmd := exec.Command("systemctl", args...)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return text, fmt.Errorf("systemctl %s: %s", strings.Join(args, " "), text)
	}
	return text, nil
}
