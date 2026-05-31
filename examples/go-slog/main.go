// Package main demonstrates publishing log/slog records to DevLogBus.
package main

import (
	"log/slog"
	"time"

	"github.com/dan-sherwin/devlogbus/pkg/sloghandler"
)

func main() {
	logger := slog.New(sloghandler.New(sloghandler.Options{
		Source: "example_go_slog",
	}))

	logger.Info("checkout started",
		slog.String("cart_id", "demo-cart"),
		slog.Int("items", 3),
	)
	logger.Warn("payment provider retry",
		slog.String("provider", "demo-pay"),
		slog.Duration("delay", 250*time.Millisecond),
	)
	logger.Error("payment declined",
		slog.String("provider", "demo-pay"),
		slog.String("reason", "insufficient_funds"),
	)

	time.Sleep(300 * time.Millisecond)
}
