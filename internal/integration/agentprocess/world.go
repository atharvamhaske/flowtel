// Package agentprocess composes the daemon and protocol test servers.
package agentprocess

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/atharvamhaske/flowtel/internal/daemon"
	"github.com/atharvamhaske/flowtel/internal/integration/inference"
	"github.com/atharvamhaske/flowtel/internal/integration/ingest"
)

type World struct {
	ctx       context.Context
	cancel    context.CancelFunc
	dataDir   string
	Daemon    *daemon.Daemon
	Inference *inference.Server
	Ingest    *ingest.Server
}

func New(parent context.Context, inferenceScenario inference.Scenario) (*World, error) {
	ctx, cancel := context.WithCancel(parent)
	dataDir, err := os.MkdirTemp("", "flowtel-agent-world-")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create agent world: %w", err)
	}
	world := &World{ctx: ctx, cancel: cancel, dataDir: dataDir, Inference: inference.New(inferenceScenario), Ingest: ingest.New()}
	sink := &httpSink{endpoint: world.Ingest.URL() + "/v1/traces"}
	config := daemon.Config{SocketPath: filepath.Join(dataDir, "daemon.sock"), DataDir: dataDir, DaemonVersion: "test", Sources: []string{"pi"}, MaxLineBytes: daemon.DefaultMaxLineBytes, QueueSize: 32}
	world.Daemon, err = daemon.New(config, sink)
	if err != nil {
		world.Close()
		return nil, fmt.Errorf("create agent daemon: %w", err)
	}
	return world, nil
}

func (w *World) Connection() (net.Conn, error) {
	if w == nil || w.Daemon == nil {
		return nil, fmt.Errorf("agent world is not ready")
	}
	serverConn, clientConn := net.Pipe()
	go w.Daemon.ServeConn(w.ctx, serverConn)
	return clientConn, nil
}

func (w *World) Close() {
	if w == nil {
		return
	}
	if w.Daemon != nil {
		_ = w.Daemon.Close()
	}
	if w.Inference != nil {
		w.Inference.Close()
	}
	if w.Ingest != nil {
		w.Ingest.Close()
	}
	w.cancel()
	if w.dataDir != "" {
		_ = os.RemoveAll(w.dataDir)
	}
}

type httpSink struct {
	endpoint string
}

func (s *httpSink) Accept(ctx context.Context, envelope daemon.Envelope) error {
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("ingest returned status %d", response.StatusCode)
	}
	return nil
}
