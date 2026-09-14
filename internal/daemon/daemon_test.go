package daemon_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atharvamhaske/flowtel/internal/daemon"
)

func TestServeSocketIsOwnerOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// AF_UNIX caps sun_path at ~104 bytes on macOS; t.TempDir() nests deep
	// enough under /var/folders to blow past that, so the socket itself
	// lives directly under /tmp instead of the per-test temp directory.
	socketPath := filepath.Join("/tmp", fmt.Sprintf("flowtel-test-%d.sock", os.Getpid()))
	t.Cleanup(func() { _ = os.Remove(socketPath) })
	config := daemon.Config{SocketPath: socketPath, DataDir: filepath.Join(root, "data"), DaemonVersion: "test", Sources: []string{"debug"}, MaxLineBytes: 4096, QueueSize: 8}
	server, err := daemon.New(config, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(ctx) }()
	// Serve() creates the socket file (net.Listen, default/umask perms)
	// and only then chmods it to 0600 — an accepted sub-millisecond gap
	// (see daemon.go's enqueue ponytail note). Poll until the permissions
	// are actually correct, not just until the file exists, or this test
	// flakes by observing that same accepted window under load.
	deadline := time.Now().Add(time.Second)
	var info os.FileInfo
	for time.Now().Before(deadline) {
		info, err = os.Stat(socketPath)
		if err == nil && info.Mode().Perm() == 0o600 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err != nil {
		select {
		case early := <-serveErr:
			t.Fatalf("stat daemon socket: %v (Serve() returned early: %v)", err, early)
		default:
		}
		t.Fatalf("stat daemon socket: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("socket permissions = %o, want 0600", perm)
	}
	cancel()
	if err := <-serveErr; err != nil {
		t.Fatalf("Serve() error = %v", err)
	}
}

// TestEventLogCapturesRealProcessAncestry is the only test in this file
// that uses a real Unix socket dial from a separate connection (like
// TestServeSocketIsOwnerOnly) rather than net.Pipe — SO_PEERCRED/
// LOCAL_PEERPID only apply to genuine AF_UNIX sockets, so every net.Pipe
// -based test in this file exercises captureAncestry's no-op path, never
// its happy path. This is the one place that proves the real capture
// actually works.
func TestEventLogCapturesRealProcessAncestry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	socketPath := filepath.Join("/tmp", fmt.Sprintf("flowtel-ancestry-test-%d.sock", os.Getpid()))
	t.Cleanup(func() { _ = os.Remove(socketPath) })
	recorded := &recordSink{}
	config := daemon.Config{SocketPath: socketPath, DataDir: filepath.Join(root, "data"), DaemonVersion: "test", Sources: []string{"debug"}, MaxLineBytes: 4096, QueueSize: 8}
	server, err := daemon.New(config, recorded)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = server.Serve(ctx) }()
	// The socket file existing doesn't guarantee the dial will succeed
	// under heavy parallel test load (transient "connection refused"
	// observed under -shuffle with many concurrent daemon tests) — retry
	// the dial itself, not just the file-existence check.
	deadline := time.Now().Add(time.Second)
	var connection net.Conn
	for time.Now().Before(deadline) {
		connection, err = net.Dial("unix", socketPath)
		if err == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("dial daemon socket: %v", err)
	}
	defer func() { _ = connection.Close() }()
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
	send(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "event.log", "params": map[string]any{"source": "debug", "session_id": "s1", "event": "tool", "payload": map[string]any{"ok": true}}})

	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) && recorded.len() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	_ = server.Close()

	recorded.mu.Lock()
	defer recorded.mu.Unlock()
	if len(recorded.envelopes) != 1 {
		t.Fatalf("recorded = %d, want 1", len(recorded.envelopes))
	}
	ancestry := recorded.envelopes[0].ProcessAncestry
	if len(ancestry) == 0 {
		t.Fatal("process ancestry is empty, want at least the dialing test process itself")
	}
	if len(ancestry) > 64 {
		t.Fatalf("ancestry depth = %d, want <= 64", len(ancestry))
	}
	if got := ancestry[0].PID; got != int32(os.Getpid()) {
		t.Fatalf("ancestry[0].PID = %d, want %d (this test process)", got, os.Getpid())
	}
}

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

// flakySink fails exactly one Accept call (by call number, across the
// whole sink lifetime) and records every envelope it accepts. It
// reproduces the exact shape of the delivery-checkpoint bug: one failure
// sitting between two successes must not let the checkpoint skip past
// the failed envelope once it eventually succeeds via retry.
type flakySink struct {
	mu        sync.Mutex
	failOn    int
	calls     int
	envelopes []daemon.Envelope
}

func (s *flakySink) Accept(_ context.Context, envelope daemon.Envelope) error {
	s.mu.Lock()
	s.calls++
	call := s.calls
	s.mu.Unlock()
	if call == s.failOn {
		return errors.New("flaky failure")
	}
	s.mu.Lock()
	s.envelopes = append(s.envelopes, envelope)
	s.mu.Unlock()
	return nil
}

func (s *flakySink) recorded() []daemon.Envelope {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]daemon.Envelope(nil), s.envelopes...)
}

// keyedFailSink fails only for one named event, permanently, and records
// everything else. Unlike flakySink (fails by call count), this lets a
// test guarantee a specific envelope is the one stuck retrying forever
// while a later envelope in the same queue would succeed if it were ever
// attempted.
type keyedFailSink struct {
	mu        sync.Mutex
	failEvent string
	envelopes []daemon.Envelope
}

func (s *keyedFailSink) Accept(_ context.Context, envelope daemon.Envelope) error {
	if envelope.Event == s.failEvent {
		return errors.New("permanently broken for this event")
	}
	s.mu.Lock()
	s.envelopes = append(s.envelopes, envelope)
	s.mu.Unlock()
	return nil
}

func (s *keyedFailSink) recorded() []daemon.Envelope {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]daemon.Envelope(nil), s.envelopes...)
}

func logNamedEvent(t *testing.T, server *daemon.Daemon, sessionID, eventName string) {
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
	send(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "event.log", "params": map[string]any{"source": "debug", "session_id": sessionID, "event": eventName, "payload": map[string]any{"ok": true}}})
}

func TestAbandonedDeliveryDoesNotLetLaterEventsCheckpointPastIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sink := &keyedFailSink{failEvent: "broken"}
	first, err := daemon.New(testConfig(root), sink)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logNamedEvent(t, first, "s1", "broken") // worker gets stuck retrying this
	logNamedEvent(t, first, "s1", "fine")   // would succeed instantly if ever attempted
	time.Sleep(50 * time.Millisecond)       // let the worker reach the backoff wait for "broken"
	if err := first.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if got := len(sink.recorded()); got != 0 {
		t.Fatalf("recorded = %d, want 0 (later event must not be delivered while an earlier one is stuck)", got)
	}

	recorded := &recordSink{}
	second, err := daemon.New(testConfig(root), recorded)
	if err != nil {
		t.Fatalf("replay New() error = %v", err)
	}
	defer func() { _ = second.Close() }()
	if !waitFlush(t, second, "s1", 2000) {
		t.Fatalf("replay flush failed: %+v", second.Status())
	}
	if got := recorded.len(); got != 2 {
		t.Fatalf("replayed = %d, want 2 (both must recover in order)", got)
	}
}

func TestDeliveryRetriesInsteadOfSkipping(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sink := &flakySink{failOn: 2}
	server, err := daemon.New(testConfig(root), sink)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = server.Close() }()
	logEvent(t, server, "s1")
	logEvent(t, server, "s1")
	logEvent(t, server, "s1")
	if !waitFlush(t, server, "s1", 2000) {
		t.Fatalf("flush failed: %+v", server.Status())
	}
	if got := len(sink.recorded()); got != 3 {
		t.Fatalf("delivered = %d, want 3 (the failed call must retry, not be dropped)", got)
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
