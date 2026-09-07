package daemon_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
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
	defer serverSide.Close()
	defer connection.Close()
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
	server.Close()
	data, err := os.ReadFile(filepath.Join(config.DataDir, "events.jsonl"))
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	if !strings.Contains(string(data), `"session_id":"s1"`) {
		t.Fatalf("journal = %s", data)
	}
}
