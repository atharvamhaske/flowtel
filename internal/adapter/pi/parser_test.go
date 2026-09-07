package pi_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/atharvamhaske/flowtel/internal/adapter/pi"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

func TestParserParse(t *testing.T) {
	input := strings.NewReader(`{"type":"session","id":"s1","timestamp":"2026-01-01T00:00:00Z"}
{"type":"message","id":"m1","parentId":"s1","sessionId":"s1","model":"model-1","provider":"provider-1","message":{"role":"assistant","usage":{"input":2}}}
{"type":"message","id":"t1","parentId":"m1","sessionId":"s1","message":{"role":"assistant","content":[{"type":"toolCall","name":"bash"}]}}
`)
	result, err := pi.NewParser(model.ProfileBoth, false).Parse(context.Background(), input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(result.Events) != 3 {
		t.Fatalf("event count = %d, want 3", len(result.Events))
	}
	if result.Events[2].ParentID != "m1" || result.Events[1].Kind != model.KindLLM {
		t.Fatalf("events = %+v", result.Events)
	}
}

func TestParserRejectsMalformedInput(t *testing.T) {
	_, err := pi.NewParser(model.ProfileBoth, false).Parse(context.Background(), strings.NewReader("{\n"))
	if !errors.Is(err, pi.ErrMalformed) {
		t.Fatalf("Parse() error = %v, want %v", err, pi.ErrMalformed)
	}
}

func TestParserBestEffortAuditsMalformedInput(t *testing.T) {
	result, err := pi.NewParser(model.ProfileBoth, true).Parse(context.Background(), strings.NewReader("{\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(result.Events) != 0 || len(result.Audit) != 1 {
		t.Fatalf("result = %+v, want one audit and no events", result)
	}
}

func TestParserReadsFlowtelBridgeEntries(t *testing.T) {
	input := strings.NewReader(`{"type":"flowtel","id":"session-1","kind":"session","name":"flowtel.session","sessionId":"session-1"}
{"type":"flowtel","id":"tool-1","parentId":"session-1","kind":"tool","name":"tool.bash","sessionId":"session-1","toolName":"bash"}
{"type":"flowtel","id":"permission-1","parentId":"tool-1","kind":"permission","name":"flowtel.permission","sessionId":"session-1","decision":"allow","source":"pi"}
`)
	result, err := pi.NewParser(model.ProfileBoth, false).Parse(context.Background(), input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(result.Events) != 3 || result.Events[2].PermissionDecision != "allow" {
		t.Fatalf("events = %+v", result.Events)
	}
}
