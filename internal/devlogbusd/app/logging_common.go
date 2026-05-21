package app

import (
	"log/slog"
	"os"
	"os/user"
	"strings"

	"github.com/dan-sherwin/devlogbus/internal/devlogbusd/app/consts"
)

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func setDefaultLogger(base *slog.Logger) {
	attrs := []any{
		slog.Int("pid", os.Getpid()),
		slog.String("app", consts.APPNAME),
		slog.String("version", consts.Version),
		slog.String("commit", consts.Commit),
		slog.String("buildDate", consts.BuildDate),
	}
	if currentUser, err := user.Current(); err == nil {
		attrs = append(attrs, slog.String("user", currentUser.Username))
	}
	slog.SetDefault(base.With(attrs...))
}
