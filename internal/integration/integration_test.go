package integration_test

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/atharvamhaske/flowtel/internal/integration/agentprocess"
	"github.com/atharvamhaske/flowtel/internal/integration/agents"
	"github.com/atharvamhaske/flowtel/internal/integration/inference"
	"github.com/atharvamhaske/flowtel/internal/integration/ingest"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

func TestPiWorldCapturesEvents(t *testing.T) {
	world, err := agentprocess.New(context.Background(), inference.Scenario{OpenAIResponse: map[string]any{"id": "mock"}})
	if err != nil {
		t.Fatalf("create world: %v", err)
	}
	defer world.Close()
	connection, err := world.Connection()
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	defer connection.Close()
	input := strings.NewReader(`{"type":"session","id":"s1"}
{"type":"message","id":"m1","sessionId":"s1","model":"test-model","provider":"test-provider","message":{"role":"assistant","usage":{"input":2,"output":1}}}
`)
	if err := (agents.Pi{Profile: model.ProfileBoth}).Run(context.Background(), connection, input); err != nil {
		t.Fatalf("run pi: %v", err)
	}
	if !world.Ingest.Matches(ingest.Scenario{Paths: []string{"/v1/traces", "/v1/logs"}}) {
		t.Fatalf("ingest rows = %+v", world.Ingest.Rows())
	}
}

func TestPiWorldSinkFailure(t *testing.T) {
	world, err := agentprocess.New(context.Background(), inference.Scenario{})
	if err != nil {
		t.Fatalf("create world: %v", err)
	}
	defer world.Close()
	world.Ingest.Fail(http.StatusInternalServerError)
	connection, err := world.Connection()
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	defer connection.Close()
	err = (agents.Pi{Profile: model.ProfileBoth, FlushMS: 1000}).Run(context.Background(), connection, strings.NewReader(piFixture))
	if err == nil {
		t.Fatal("run pi: error = nil, want sink failure")
	}
	if world.Daemon.Status().LastError == "" {
		t.Fatalf("status = %+v, want last_error", world.Daemon.Status())
	}
}

func TestPiWorldFlushTimeout(t *testing.T) {
	world, err := agentprocess.New(context.Background(), inference.Scenario{})
	if err != nil {
		t.Fatalf("create world: %v", err)
	}
	defer world.Close()
	world.Ingest.Stall(400 * time.Millisecond)
	connection, err := world.Connection()
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	defer connection.Close()
	err = (agents.Pi{Profile: model.ProfileBoth, FlushMS: 50}).Run(context.Background(), connection, strings.NewReader(piFixture))
	if err == nil {
		t.Fatal("run pi: error = nil, want flush timeout")
	}
}

func TestPiWorldShutdownDrains(t *testing.T) {
	world, err := agentprocess.New(context.Background(), inference.Scenario{})
	if err != nil {
		t.Fatalf("create world: %v", err)
	}
	connection, err := world.Connection()
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	if err := (agents.Pi{Profile: model.ProfileBoth}).Run(context.Background(), connection, strings.NewReader(piFixture)); err != nil {
		connection.Close()
		world.Close()
		t.Fatalf("run pi: %v", err)
	}
	connection.Close()
	world.Close()
	if world.Daemon.Status().Queued != 0 {
		t.Fatalf("queued = %d, want 0 after shutdown", world.Daemon.Status().Queued)
	}
	if !world.Ingest.Matches(ingest.Scenario{Paths: []string{"/v1/traces", "/v1/logs"}}) {
		t.Fatalf("ingest rows = %+v", world.Ingest.Rows())
	}
}

func TestPiProcessCapturesEvents(t *testing.T) {
	if _, err := agentprocess.PiBinary(); err != nil {
		t.Skip("pi is not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	world, err := agentprocess.New(ctx, inference.Scenario{})
	if err != nil {
		t.Fatalf("create world: %v", err)
	}
	defer world.Close()
	output, err := world.RunPi(ctx, "Say hi")
	if err != nil {
		t.Fatalf("run pi process: %v", err)
	}
	connection, err := world.Connection()
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	defer connection.Close()
	if err := (agents.Pi{Profile: model.ProfileBoth, FlushMS: 5000}).Run(ctx, connection, bytes.NewReader(output)); err != nil {
		t.Fatalf("forward pi events: %v\nstdout=%s", err, output)
	}
	if !world.Ingest.Matches(ingest.Scenario{Paths: []string{"/v1/traces", "/v1/logs"}}) {
		t.Fatalf("ingest rows = %+v\nstdout=%s", world.Ingest.Rows(), output)
	}
}

const piFixture = `{"type":"session","id":"s1"}
{"type":"message","id":"m1","sessionId":"s1","model":"test-model","provider":"test-provider","message":{"role":"assistant","usage":{"input":2,"output":1}}}
`
