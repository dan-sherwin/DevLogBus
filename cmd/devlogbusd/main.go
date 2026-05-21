package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/dan-sherwin/devlogbus/internal/devlogbusd/app"
)

func main() {
	app.Setup()
	if err := app.CLICommand.Run(); err != nil {
		slog.Error("error running command", slog.String("error", err.Error()))
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
