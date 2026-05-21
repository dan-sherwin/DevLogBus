package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/rpc"
	"os"
	"path/filepath"

	"github.com/dan-sherwin/devlogbus/pkg/client"
)

var SettingsRPCSocketPath = filepath.Join(filepath.Dir(client.DefaultSocketPath()), "devlogbusd-settings.sock")

func startSettingsRPCServer(ctx context.Context) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(SettingsRPCSocketPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.Remove(SettingsRPCSocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	listener, err := net.Listen("unix", SettingsRPCSocketPath)
	if err != nil {
		return nil, err
	}

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Debug("settings rpc accept failed", slog.String("error", err.Error()))
				continue
			}
			go rpc.ServeConn(conn)
		}
	}()

	cleanup := func() {
		_ = listener.Close()
		_ = os.Remove(SettingsRPCSocketPath)
	}
	return cleanup, nil
}
