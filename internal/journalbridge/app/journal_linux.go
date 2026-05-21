//go:build linux

package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type journalStreamOptions struct {
	Since   string
	Tail    uint64
	Once    bool
	Matches []string
}

func streamJournal(ctx context.Context, options journalStreamOptions, handle func(journalEntry) error) error {
	args, err := journalctlArgs(options)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "journalctl", args...)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		entry, err := parseJournalctlJSON(scanner.Bytes())
		if err != nil {
			return err
		}
		if err := handle(entry); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		_ = cmd.Wait()
		return err
	}
	if err := cmd.Wait(); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}

func journalctlArgs(options journalStreamOptions) ([]string, error) {
	args := []string{"--output=json", "--no-pager"}
	if !options.Once {
		args = append(args, "--follow")
	}

	if options.Tail > 0 {
		args = append(args, "-n", strconv.FormatUint(options.Tail, 10))
	} else {
		since := strings.ToLower(strings.TrimSpace(options.Since))
		switch since {
		case "", "now", "tail":
			args = append(args, "--since=now")
		case "all", "head", "beginning":
		default:
			duration, err := time.ParseDuration(since)
			if err != nil {
				return nil, fmt.Errorf("since must be now, all, or a duration like 10m: %w", err)
			}
			if duration < 0 {
				duration = -duration
			}
			args = append(args, "--since="+time.Now().Add(-duration).Format("2006-01-02 15:04:05"))
		}
	}

	for _, match := range options.Matches {
		if !strings.Contains(match, "=") {
			return nil, fmt.Errorf("journal match must be FIELD=VALUE: %q", match)
		}
		args = append(args, match)
	}
	return args, nil
}

func parseJournalctlJSON(line []byte) (journalEntry, error) {
	var raw map[string]any
	if err := json.Unmarshal(line, &raw); err != nil {
		return journalEntry{}, err
	}

	fields := make(map[string]string, len(raw))
	for key, value := range raw {
		if fieldValue := journalFieldString(value); fieldValue != "" {
			fields[key] = fieldValue
		}
	}
	return journalEntry{
		Fields:             fields,
		Cursor:             fields["__CURSOR"],
		RealtimeTimestamp:  parseJournalUint(fields["__REALTIME_TIMESTAMP"]),
		MonotonicTimestamp: parseJournalUint(fields["__MONOTONIC_TIMESTAMP"]),
	}, nil
}

func journalFieldString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case []any:
		values := make([]string, 0, len(v))
		for _, item := range v {
			if itemValue := journalFieldString(item); itemValue != "" {
				values = append(values, itemValue)
			}
		}
		return strings.Join(values, ",")
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(data)
	}
}

func parseJournalUint(value string) uint64 {
	parsed, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	return parsed
}
