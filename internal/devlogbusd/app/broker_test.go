package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dan-sherwin/devlogbus/pkg/client"
	"github.com/dan-sherwin/devlogbus/pkg/protocol"
)

func TestPublishRetainsMaxRecordsPerSource(t *testing.T) {
	b := &broker{
		maxRecords:  2,
		subscribers: map[int]subscriber{},
	}
	now := time.Now()

	b.publish(protocol.Record{Time: now, Level: "info", Source: "quiet", Message: "quiet first"})
	b.publish(protocol.Record{Time: now, Level: "info", Source: "noisy", Message: "noisy first"})
	b.publish(protocol.Record{Time: now, Level: "info", Source: "noisy", Message: "noisy second"})
	b.publish(protocol.Record{Time: now, Level: "info", Source: "noisy", Message: "noisy third"})

	all := b.replay(protocol.Subscribe{})
	if len(all) != 3 {
		t.Fatalf("replay len = %d, want 3", len(all))
	}
	if all[0].Source != "quiet" || all[0].Message != "quiet first" {
		t.Fatalf("first record = %#v, want quiet record preserved", all[0])
	}
	if all[1].Message != "noisy second" || all[2].Message != "noisy third" {
		t.Fatalf("noisy records = %#v, want last two noisy records", all[1:])
	}

	quiet := b.replay(protocol.Subscribe{Sources: []string{"quiet"}})
	if len(quiet) != 1 || quiet[0].Message != "quiet first" {
		t.Fatalf("quiet replay = %#v, want quiet first retained", quiet)
	}
}

func TestReplayPerSourceLimitsEachSource(t *testing.T) {
	b := &broker{
		maxRecords:  10,
		subscribers: map[int]subscriber{},
	}
	now := time.Now()

	for i := 1; i <= 4; i++ {
		b.publish(protocol.Record{Time: now.Add(time.Duration(i) * time.Millisecond), Level: "info", Source: "noisy", Message: fmt.Sprintf("noisy %d", i)})
		if i <= 3 {
			b.publish(protocol.Record{Time: now.Add(time.Duration(i+10) * time.Millisecond), Level: "info", Source: "quiet", Message: fmt.Sprintf("quiet %d", i)})
		}
	}

	records := b.replay(protocol.Subscribe{ReplayPerSource: 2})
	if len(records) != 4 {
		t.Fatalf("replay len = %d, want 4", len(records))
	}
	got := []string{
		records[0].Message,
		records[1].Message,
		records[2].Message,
		records[3].Message,
	}
	want := []string{"quiet 2", "noisy 3", "quiet 3", "noisy 4"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("records = %#v, want %#v", got, want)
		}
	}
}

func TestExpungeRemovesSourceOrAllRecords(t *testing.T) {
	b := &broker{
		maxRecords:  10,
		subscribers: map[int]subscriber{},
	}
	now := time.Now()

	b.publish(protocol.Record{Time: now, Level: "info", Source: "quiet", Message: "quiet first"})
	b.publish(protocol.Record{Time: now, Level: "info", Source: "noisy", Message: "noisy first"})
	b.publish(protocol.Record{Time: now, Level: "info", Source: "noisy", Message: "noisy second"})

	if expunged := b.expunge("noisy"); expunged != 2 {
		t.Fatalf("expunged = %d, want 2", expunged)
	}
	records := b.replay(protocol.Subscribe{})
	if len(records) != 1 || records[0].Source != "quiet" {
		t.Fatalf("records after source expunge = %#v, want quiet only", records)
	}

	if expunged := b.expunge(""); expunged != 1 {
		t.Fatalf("expunged all = %d, want 1", expunged)
	}
	if records := b.replay(protocol.Subscribe{}); len(records) != 0 {
		t.Fatalf("records after full expunge = %#v, want none", records)
	}
}

func TestClientExpungeRemovesBrokerReplayRecords(t *testing.T) {
	b := &broker{
		maxRecords:  10,
		subscribers: map[int]subscriber{},
	}
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("devlogbus-test-%d.sock", time.Now().UnixNano()))
	listeners, err := openBrokerListeners(socketPath, "")
	if err != nil {
		t.Fatalf("open listeners: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for _, listener := range listeners {
		listener := listener
		wg.Add(1)
		go func() {
			defer wg.Done()
			serveBrokerListener(ctx, listener, b)
		}()
	}
	t.Cleanup(func() {
		cancel()
		for _, listener := range listeners {
			_ = listener.Close()
			if listener.cleanup != nil {
				listener.cleanup()
			}
		}
		wg.Wait()
	})

	b.publish(protocol.Record{Time: time.Now(), Level: "info", Source: "quiet", Message: "quiet first"})
	b.publish(protocol.Record{Time: time.Now(), Level: "info", Source: "noisy", Message: "noisy first"})
	b.publish(protocol.Record{Time: time.Now(), Level: "info", Source: "noisy", Message: "noisy second"})

	expungeCtx, expungeCancel := context.WithTimeout(context.Background(), time.Second)
	defer expungeCancel()
	brokerClient := client.New(socketPath)
	expunged, err := brokerClient.Expunge(expungeCtx, "noisy")
	if err != nil {
		t.Fatalf("expunge noisy: %v", err)
	}
	if expunged != 2 {
		t.Fatalf("expunged = %d, want 2", expunged)
	}

	records := b.replay(protocol.Subscribe{})
	if len(records) != 1 || records[0].Source != "quiet" {
		t.Fatalf("records after client expunge = %#v, want quiet only", records)
	}
}

func TestUnixBrokerSocketIsWritableByLocalClients(t *testing.T) {
	socketPath := filepath.Join("/tmp", fmt.Sprintf("devlogbus-test-%d.sock", time.Now().UnixNano()))
	listener, err := openBrokerListener(socketPath)
	if err != nil {
		t.Fatalf("open listener: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
		if listener.cleanup != nil {
			listener.cleanup()
		}
	})

	info, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o666 {
		t.Fatalf("socket permissions = %#o, want 0666", got)
	}
}

func TestTCPBrokerPublishAndSubscribe(t *testing.T) {
	b := &broker{
		maxRecords:  10,
		subscribers: map[int]subscriber{},
	}
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("devlogbus-test-%d.sock", time.Now().UnixNano()))
	listeners, err := openBrokerListeners(socketPath, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("open listeners: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for _, listener := range listeners {
		listener := listener
		wg.Add(1)
		go func() {
			defer wg.Done()
			serveBrokerListener(ctx, listener, b)
		}()
	}
	t.Cleanup(func() {
		cancel()
		for _, listener := range listeners {
			_ = listener.Close()
			if listener.cleanup != nil {
				listener.cleanup()
			}
		}
		wg.Wait()
	})

	var tcpAddress string
	for _, listener := range listeners {
		if listener.network == client.NetworkTCP {
			tcpAddress = listener.address
			break
		}
	}
	if tcpAddress == "" {
		t.Fatalf("tcp listener was not opened")
	}

	brokerClient := client.NewWithOptions(client.Options{
		Network: client.NetworkTCP,
		Address: tcpAddress,
	})
	publishCtx, publishCancel := context.WithTimeout(context.Background(), time.Second)
	defer publishCancel()
	if err := brokerClient.Publish(publishCtx, protocol.Record{
		Time:    time.Now(),
		Level:   "warn",
		Source:  "tcp-test",
		Message: "remote publish",
	}); err != nil {
		t.Fatalf("publish over tcp: %v", err)
	}

	subscribeCtx, subscribeCancel := context.WithTimeout(context.Background(), time.Second)
	defer subscribeCancel()
	sub, err := brokerClient.Subscribe(subscribeCtx, protocol.Subscribe{Replay: 1})
	if err != nil {
		t.Fatalf("subscribe over tcp: %v", err)
	}
	defer func() { _ = sub.Close() }()

	select {
	case record := <-sub.Records:
		if record.Source != "tcp-test" || record.Message != "remote publish" {
			t.Fatalf("record = %#v, want tcp-test remote publish", record)
		}
	case err := <-sub.Errors:
		t.Fatalf("subscription error: %v", err)
	case <-subscribeCtx.Done():
		t.Fatalf("timed out waiting for replayed record")
	}

	select {
	case <-sub.ReplayDone:
	case err := <-sub.Errors:
		t.Fatalf("subscription error waiting for replay completion: %v", err)
	case <-subscribeCtx.Done():
		t.Fatalf("timed out waiting for replay completion")
	}
}

func TestEndpointCanBeTCPAddress(t *testing.T) {
	b := &broker{
		maxRecords:  10,
		subscribers: map[int]subscriber{},
	}
	listeners, err := openBrokerListeners("127.0.0.1:0", "")
	if err != nil {
		t.Fatalf("open listeners: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for _, listener := range listeners {
		listener := listener
		wg.Add(1)
		go func() {
			defer wg.Done()
			serveBrokerListener(ctx, listener, b)
		}()
	}
	t.Cleanup(func() {
		cancel()
		for _, listener := range listeners {
			_ = listener.Close()
			if listener.cleanup != nil {
				listener.cleanup()
			}
		}
		wg.Wait()
	})

	if len(listeners) != 1 {
		t.Fatalf("listener count = %d, want 1", len(listeners))
	}
	if listeners[0].network != client.NetworkTCP {
		t.Fatalf("listener network = %q, want tcp", listeners[0].network)
	}

	brokerClient := client.New(listeners[0].address)
	publishCtx, publishCancel := context.WithTimeout(context.Background(), time.Second)
	defer publishCancel()
	if err := brokerClient.Publish(publishCtx, protocol.Record{
		Time:    time.Now(),
		Level:   "info",
		Source:  "tcp-endpoint-test",
		Message: "endpoint publish",
	}); err != nil {
		t.Fatalf("publish over endpoint tcp address: %v", err)
	}

	deadline := time.After(time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		records := b.replay(protocol.Subscribe{Sources: []string{"tcp-endpoint-test"}})
		if len(records) == 1 && records[0].Message == "endpoint publish" {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("records = %#v, want endpoint publish", records)
		case <-tick.C:
		}
	}
}
