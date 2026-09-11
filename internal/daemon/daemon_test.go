package daemon_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atharvamhaske/flowtel/internal/daemon"
)

func TestDaemonWireProtocol(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	config := daemon.Config{SocketPath: filepath.Join(root, "run", "daemon.sock"), DataDir: filepath.Join(root, "data"), DaemonVersion: "test", Sources: []string{"debug"}, MaxLineBytes: 4096, QueueSize: 8}
	server, err := daemon.New(config, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverSide, connection := net.Pipe()
	defer func() { _ = serverSide.Close() }()
	defer func() { _ = connection.Close() }()
	go server.ServeConn(ctx, serverSide)
	scanner := bufio.NewScanner(connection)
	send := func(request map[string]any) map[string]any {
		t.Helper()
		encoded, marshalErr := json.Marshal(request)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if _, writeErr := connection.Write(append(encoded, '\n')); writeErr != nil {
			t.Fatal(writeErr)
		}
		if !scanner.Scan() {
			t.Fatalf("response missing: %v", scanner.Err())
		}
		var response map[string]any
		if decodeErr := json.Unmarshal(scanner.Bytes(), &response); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		return response
	}
	initResponse := send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocol_version": 1, "client": map[string]any{"source": "debug"}}})
	if initResponse["error"] != nil {
		t.Fatalf("initialize response = %v", initResponse)
	}
	eventResponse := send(map[string]any{"jsonrpc": "2.0", "id": "event", "method": "event.log", "params": map[string]any{"source": "debug", "source_version": "1", "plugin_version": "1", "session_id": "s1", "event": "tool", "ts_ms": time.Now().UnixMilli(), "payload": map[string]any{"kind": "tool"}, "config": map[string]any{"profile": "both"}, "capture": map[string]any{"pid": 1}}})
	if eventResponse["error"] != nil {
		t.Fatalf("event response = %v", eventResponse)
	}
	statusResponse := send(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "status.get"})
	result, ok := statusResponse["result"].(map[string]any)
	if !ok || result["events_stored"] != float64(1) {
		t.Fatalf("status response = %v", statusResponse)
	}
	shutdownResponse := send(map[string]any{"jsonrpc": "2.0", "id": 3, "method": "daemon.shutdown"})
	if shutdownResponse["error"] != nil {
		t.Fatalf("shutdown response = %v", shutdownResponse)
	}
	_ = server.Close()
	data, err := os.ReadFile(filepath.Join(config.DataDir, "events.jsonl"))
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	if !strings.Contains(string(data), `"session_id":"s1"`) {
		t.Fatalf("journal = %s", data)
	}
}

type recordSink struct {
	mu        sync.Mutex
	envelopes []daemon.Envelope
}

func (s *recordSink) Accept(_ context.Context, envelope daemon.Envelope) error {
	s.mu.Lock()
	s.envelopes = append(s.envelopes, envelope)
	s.mu.Unlock()
	return nil
}

func (s *recordSink) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.envelopes)
}

type failSink struct{ err error }

func (s failSink) Accept(context.Context, daemon.Envelope) error { return s.err }

type slowSink struct{ delay time.Duration }

func (s slowSink) Accept(ctx context.Context, envelope daemon.Envelope) error {
	select {
	case <-time.After(s.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestReplayDeliversPending(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	config := testConfig(root)
	first, err := daemon.New(config, failSink{err: errors.New("boom")})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logEvent(t, first, "s1")
	if waitFlush(t, first, "s1", 1000) {
		t.Fatal("first flush succeeded, want sink failure")
	}
	_ = first.Close()
	recorded := &recordSink{}
	second, err := daemon.New(config, recorded)
	if err != nil {
		t.Fatalf("replay New() error = %v", err)
	}
	if !waitFlush(t, second, "s1", 1000) {
		t.Fatalf("replay flush failed: %+v", second.Status())
	}
	_ = second.Close()
	if recorded.len() != 1 {
		t.Fatalf("replayed = %d, want 1", recorded.len())
	}
}

func TestReplaySkipsDelivered(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	config := testConfig(root)
	first, err := daemon.New(config, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logEvent(t, first, "s1")
	if !waitFlush(t, first, "s1", 1000) {
		t.Fatalf("first flush failed: %+v", first.Status())
	}
	_ = first.Close()
	recorded := &recordSink{}
	second, err := daemon.New(config, recorded)
	if err != nil {
		t.Fatalf("replay New() error = %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	_ = second.Close()
	if recorded.len() != 0 {
		t.Fatalf("replayed = %d, want 0", recorded.len())
	}
}

func TestFlushTimeout(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	server, err := daemon.New(testConfig(root), slowSink{delay: 400 * time.Millisecond})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = server.Close() }()
	logEvent(t, server, "s1")
	if waitFlush(t, server, "s1", 50) {
		t.Fatal("flush succeeded, want timeout")
	}
	if server.Status().Sessions[0].Queued < 1 && server.Status().LastError != "" {
		t.Fatalf("status = %+v", server.Status())
	}
}

func TestShutdownDrains(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	recorded := &recordSink{}
	server, err := daemon.New(testConfig(root), recorded)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logEvent(t, server, "s1")
	logEvent(t, server, "s1")
	if err := server.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if recorded.len() != 2 {
		t.Fatalf("drained = %d, want 2", recorded.len())
	}
}

func TestSinkFailureVisible(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	server, err := daemon.New(testConfig(root), failSink{err: errors.New("http 500")})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = server.Close() }()
	logEvent(t, server, "s1")
	if waitFlush(t, server, "s1", 1000) {
		t.Fatal("flush succeeded, want sink failure")
	}
	if server.Status().LastError == "" {
		t.Fatalf("status = %+v, want last_error", server.Status())
	}
}

func testConfig(root string) daemon.Config {
	return daemon.Config{SocketPath: filepath.Join(root, "run", "daemon.sock"), DataDir: filepath.Join(root, "data"), DaemonVersion: "test", Sources: []string{"debug"}, MaxLineBytes: 4096, QueueSize: 8}
}

func logEvent(t *testing.T, server *daemon.Daemon, sessionID string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverSide, connection := net.Pipe()
	defer func() { _ = serverSide.Close() }()
	defer func() { _ = connection.Close() }()
	go server.ServeConn(ctx, serverSide)
	scanner := bufio.NewScanner(connection)
	send := func(request map[string]any) {
		t.Helper()
		encoded, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := connection.Write(append(encoded, '\n')); err != nil {
			t.Fatal(err)
		}
		if !scanner.Scan() {
			t.Fatalf("response missing: %v", scanner.Err())
		}
	}
	send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocol_version": 1, "client": map[string]any{"source": "debug"}}})
	send(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "event.log", "params": map[string]any{"source": "debug", "session_id": sessionID, "event": "tool", "payload": map[string]any{"ok": true}}})
}

func waitFlush(t *testing.T, server *daemon.Daemon, sessionID string, timeoutMS int) bool {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverSide, connection := net.Pipe()
	defer func() { _ = serverSide.Close() }()
	defer func() { _ = connection.Close() }()
	go server.ServeConn(ctx, serverSide)
	scanner := bufio.NewScanner(connection)
	send := func(request map[string]any) map[string]any {
		t.Helper()
		encoded, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := connection.Write(append(encoded, '\n')); err != nil {
			t.Fatal(err)
		}
		if !scanner.Scan() {
			t.Fatalf("response missing: %v", scanner.Err())
		}
		var response map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response
	}
	send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocol_version": 1, "client": map[string]any{"source": "debug"}}})
	response := send(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "session.flush", "params": map[string]any{"session_id": sessionID, "timeout_ms": timeoutMS}})
	result, _ := response["result"].(map[string]any)
	flushed, _ := result["flushed"].(bool)
	return flushed
}
