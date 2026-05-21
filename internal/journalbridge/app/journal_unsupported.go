//go:build !linux

package app

import (
	"context"
	"fmt"
	"runtime"
)

type journalStreamOptions struct {
	Since   string
	Tail    uint64
	Once    bool
	Matches []string
}

func streamJournal(_ context.Context, _ journalStreamOptions, _ func(journalEntry) error) error {
	return fmt.Errorf("journald bridge is only supported on linux; current platform is %s", runtime.GOOS)
}
