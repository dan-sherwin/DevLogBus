//go:build !linux

package app

import (
	"log/slog"
	"os"
)

func initLogger() {
	level := parseLevel(LoggingLevel)
	setDefaultLogger(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}
