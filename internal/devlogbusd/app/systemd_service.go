package app

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/dan-sherwin/devlogbus/internal/devlogbusd/app/consts"
	"github.com/takama/daemon"
)

type (
	SystemService struct {
		daemon.Daemon
	}
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

var systemdService *SystemService

func setupSystemdService() {
	kind := daemon.SystemDaemon
	if runtime.GOOS == "darwin" {
		kind = daemon.GlobalDaemon
	}
	srv, err := daemon.New(consts.APPNAME, "DevLogBus local structured log broker", kind)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	systemdService = &SystemService{Daemon: srv}
}

func (i *InstallServiceCommand) Run() error {
	status, err := systemdService.Install("run")
	if err != nil {
		slog.Error("service install failed", slog.String("error", err.Error()))
		return err
	}
	fmt.Println(status)
	return nil
}

func (r *RemoveServiceCommand) Run() error {
	status, err := systemdService.Remove()
	if err != nil {
		slog.Error("service remove failed", slog.String("error", err.Error()))
		return err
	}
	fmt.Println(status)
	return nil
}

func (s *StartServiceCommand) Run() error {
	status, err := systemdService.Start()
	if err != nil {
		slog.Error("service start failed", slog.String("error", err.Error()))
		return err
	}
	fmt.Println(status)
	return nil
}

func (s *StopServiceCommand) Run() error {
	status, err := systemdService.Stop()
	if err != nil {
		slog.Error("service stop failed", slog.String("error", err.Error()))
		return err
	}
	fmt.Println(status)
	return nil
}

func (r *RestartServiceCommand) Run() error {
	statuses, err := systemdService.ReStart()
	if err != nil {
		slog.Error("service restart failed", slog.String("error", err.Error()))
		return err
	}
	for _, status := range statuses {
		fmt.Println(status)
	}
	return nil
}

func (s *ServiceStatusCommand) Run() error {
	status, err := systemdService.Status()
	if err != nil {
		slog.Error("service status failed", slog.String("error", err.Error()))
		return err
	}
	fmt.Println(status)
	return nil
}

func (s *SystemService) ReStart() ([]string, error) {
	stopStatus, err := s.Stop()
	if err != nil {
		return nil, err
	}
	startStatus, err := s.Start()
	if err != nil {
		return nil, err
	}
	return []string{stopStatus, startStatus}, nil
}
