package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/dan-sherwin/devlogbus/internal/recordfmt"
	"github.com/dan-sherwin/devlogbus/pkg/client"
	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

type (
	repeatedStrings []string
	EmitCommand     struct {
		SocketPath string          `name:"socket" default:"${socket_path}" help:"Unix socket path"`
		Source     string          `name:"source" default:"devlogbus" help:"Record source"`
		Level      string          `name:"level" default:"info" help:"Record level"`
		Message    string          `name:"message" default:"test log record" help:"Record message"`
		Attrs      repeatedStrings `name:"attr" help:"Attribute in key=value form; repeatable"`
	}
	TailCommand struct {
		SocketPath string          `name:"socket" default:"${socket_path}" help:"Unix socket path"`
		Level      string          `name:"level" default:"debug" help:"Minimum level"`
		Replay     int             `name:"replay" default:"100" help:"Number of matching records to replay"`
		Sources    repeatedStrings `name:"source" help:"Source to include; repeatable"`
	}
	SocketCommand struct {
		SocketPath string `name:"socket" default:"${socket_path}" help:"Unix socket path"`
	}
)

func (c *EmitCommand) Run() error {
	record := protocol.Record{
		Time:    time.Now(),
		Level:   c.Level,
		Source:  c.Source,
		Message: c.Message,
		Attrs:   parseAttrs(c.Attrs),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return client.New(c.SocketPath).Publish(ctx, record)
}

func (c *TailCommand) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	sub, err := client.New(c.SocketPath).Subscribe(ctx, protocol.Subscribe{
		Sources:  c.Sources,
		MinLevel: c.Level,
		Replay:   c.Replay,
	})
	if err != nil {
		return err
	}
	defer func() { _ = sub.Close() }()

	for {
		select {
		case <-ctx.Done():
			return nil
		case record, ok := <-sub.Records:
			if !ok {
				return nil
			}
			fmt.Println(recordfmt.Format(record))
		case err := <-sub.Errors:
			if err != nil {
				slog.Error("tail failed", slog.String("error", err.Error()))
				return err
			}
		}
	}
}

func (c *SocketCommand) Run() error {
	fmt.Println(c.SocketPath)
	return nil
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
