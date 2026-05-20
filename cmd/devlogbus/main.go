package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/dan-sherwin/devlogbus/pkg/client"
	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

type repeatedStrings []string

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "emit":
		runEmit(os.Args[2:])
	case "tail":
		runTail(os.Args[2:])
	case "socket":
		fmt.Println(client.DefaultSocketPath())
	default:
		usage()
		os.Exit(2)
	}
}

func runEmit(args []string) {
	fs := flag.NewFlagSet("emit", flag.ExitOnError)
	socketPath := fs.String("socket", client.DefaultSocketPath(), "Unix socket path")
	source := fs.String("source", "devlogbus", "record source")
	level := fs.String("level", "info", "record level")
	message := fs.String("message", "test log record", "record message")
	var attrs repeatedStrings
	fs.Var(&attrs, "attr", "attribute in key=value form; repeatable")
	_ = fs.Parse(args)

	record := protocol.Record{
		Time:    time.Now(),
		Level:   *level,
		Source:  *source,
		Message: *message,
		Attrs:   parseAttrs(attrs),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.New(*socketPath).Publish(ctx, record); err != nil {
		slog.Error("emit failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func runTail(args []string) {
	fs := flag.NewFlagSet("tail", flag.ExitOnError)
	socketPath := fs.String("socket", client.DefaultSocketPath(), "Unix socket path")
	level := fs.String("level", "debug", "minimum level")
	replay := fs.Int("replay", 100, "number of matching records to replay")
	var sources repeatedStrings
	fs.Var(&sources, "source", "source to include; repeatable")
	_ = fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	sub, err := client.New(*socketPath).Subscribe(ctx, protocol.Subscribe{
		Sources:  sources,
		MinLevel: *level,
		Replay:   *replay,
	})
	if err != nil {
		slog.Error("subscribe failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer sub.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case record, ok := <-sub.Records:
			if !ok {
				return
			}
			fmt.Println(formatRecord(record))
		case err := <-sub.Errors:
			if err != nil {
				slog.Error("tail failed", slog.String("error", err.Error()))
				os.Exit(1)
			}
		}
	}
}

func usage() {
	fmt.Println("usage: devlogbus <emit|tail|socket> [flags]")
}

func (r *repeatedStrings) String() string {
	return strings.Join(*r, ",")
}

func (r *repeatedStrings) Set(value string) error {
	*r = append(*r, value)
	return nil
}

func parseAttrs(values []string) map[string]any {
	attrs := map[string]any{}
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		if !ok || key == "" {
			continue
		}
		attrs[key] = val
	}
	if len(attrs) == 0 {
		return nil
	}
	return attrs
}

func formatRecord(record protocol.Record) string {
	keys := make([]string, 0, len(record.Attrs))
	for key := range record.Attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	fields := make([]string, 0, len(keys))
	for _, key := range keys {
		fields = append(fields, fmt.Sprintf("%s=%v", key, record.Attrs[key]))
	}
	suffix := ""
	if len(fields) > 0 {
		suffix = " " + strings.Join(fields, " ")
	}
	return fmt.Sprintf("%s %-5s %-24s %s%s", record.Time.Format("15:04:05.000"), protocol.NormalizeLevel(record.Level), record.Source, record.Message, suffix)
}
