package integration_test

import (
	"context"
	"strings"
	"testing"

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
	if !world.Ingest.Matches(ingest.Scenario{Paths: []string{"/v1/traces", "/v1/traces"}}) {
		t.Fatalf("ingest rows = %+v", world.Ingest.Rows())
	}
}
