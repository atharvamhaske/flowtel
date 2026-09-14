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

func TestParserCapturesCacheWriteAndCost(t *testing.T) {
	// Real usage shape from an actual pi session: usage.cacheWrite and a
	// nested usage.cost block, neither of which the parser used to read.
	input := strings.NewReader(`{"type":"session","id":"s1","timestamp":"2026-01-01T00:00:00Z"}
{"type":"message","id":"m1","parentId":"s1","sessionId":"s1","model":"model-1","provider":"provider-1","message":{"role":"assistant","usage":{"input":2,"output":1,"cacheRead":0,"cacheWrite":5,"cost":{"input":0.001,"output":0.002,"cacheRead":0,"cacheWrite":0.0005,"total":0.0035}}}}
`)
	result, err := pi.NewParser(model.ProfileBoth, false).Parse(context.Background(), input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(result.Events) != 2 {
		t.Fatalf("event count = %d, want 2", len(result.Events))
	}
	event := result.Events[1]
	if event.CacheWriteTokens != 5 {
		t.Fatalf("CacheWriteTokens = %d, want 5", event.CacheWriteTokens)
	}
	if event.CostInput != 0.001 || event.CostOutput != 0.002 || event.CostCacheWrite != 0.0005 || event.CostTotal != 0.0035 {
		t.Fatalf("cost fields = input=%v output=%v cacheWrite=%v total=%v, want 0.001/0.002/0.0005/0.0035",
			event.CostInput, event.CostOutput, event.CostCacheWrite, event.CostTotal)
	}
}

func TestParserCapturesLLMError(t *testing.T) {
	// Real shape observed from an actual pi session where a provider
	// call failed (github-copilot returning 421 Misdirected Request).
	input := strings.NewReader(`{"type":"session","id":"s1","timestamp":"2026-01-01T00:00:00Z"}
{"type":"message","id":"m1","parentId":"s1","sessionId":"s1","model":"model-1","provider":"provider-1","message":{"role":"assistant","usage":{"input":2},"stopReason":"error","errorMessage":"421 Misdirected Request"}}
`)
	result, err := pi.NewParser(model.ProfileBoth, false).Parse(context.Background(), input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(result.Events) != 2 {
		t.Fatalf("event count = %d, want 2", len(result.Events))
	}
	if result.Events[1].Kind != model.KindLLM {
		t.Fatalf("events[1].Kind = %v, want KindLLM", result.Events[1].Kind)
	}
	if result.Events[1].Error != "421 Misdirected Request" {
		t.Fatalf("events[1].Error = %q, want %q", result.Events[1].Error, "421 Misdirected Request")
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

func TestParserReadsPiLifecycleEvents(t *testing.T) {
	input := strings.NewReader(`{"event":"session_start","session_id":"s1","ts_ms":1}
{"event":"message_end","session_id":"s1","ts_ms":2,"message":{"role":"assistant","model":"gpt-5","provider":"openai","usage":{"input":5,"output":2}}}
{"event":"tool_execution_start","session_id":"s1","ts_ms":3,"toolCallId":"call-1","toolName":"read"}
{"event":"tool_execution_end","session_id":"s1","ts_ms":4,"toolCallId":"call-1","toolName":"read","isError":true}
`)
	result, err := pi.NewParser(model.ProfileBoth, false).Parse(context.Background(), input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(result.Events) != 4 {
		t.Fatalf("event count = %d, want 4", len(result.Events))
	}
	if result.Events[1].Kind != model.KindLLM || result.Events[1].InputTokens != 5 || result.Events[1].OutputTokens != 2 {
		t.Fatalf("llm event = %+v", result.Events[1])
	}
	if result.Events[3].ID != "call-1" || result.Events[3].Error == "" {
		t.Fatalf("tool event = %+v", result.Events[3])
	}
}

func TestParserReadsPiAgentTurnCompaction(t *testing.T) {
	input := strings.NewReader(`{"type":"session","id":"s1"}
{"type":"agent_start","session_id":"s1"}
{"type":"turn_start","session_id":"s1"}
{"type":"message_end","session_id":"s1","message":{"role":"assistant","model":"gpt-5","usage":{"input":3,"output":1,"cacheRead":2,"reasoning":4}}}
{"type":"tool_execution_start","session_id":"s1","toolCallId":"c1","toolName":"bash"}
{"type":"tool_execution_end","session_id":"s1","toolCallId":"c1","toolName":"bash","isError":true,"stderr":"boom"}
{"type":"compaction_end","session_id":"s1"}
{"type":"turn_end","session_id":"s1"}
{"type":"agent_end","session_id":"s1"}
`)
	result, err := pi.NewParser(model.ProfileBoth, false).Parse(context.Background(), input)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(result.Events) != 9 {
		t.Fatalf("event count = %d, want 9", len(result.Events))
	}
	session, agent, turn, llm, toolStart, toolEnd, compaction := result.Events[0], result.Events[1], result.Events[2], result.Events[3], result.Events[4], result.Events[5], result.Events[6]
	if agent.Kind != model.KindAgent || agent.ParentID != session.ID {
		t.Fatalf("agent = %+v", agent)
	}
	if turn.Kind != model.KindTurn || turn.ParentID != agent.ID {
		t.Fatalf("turn = %+v", turn)
	}
	if llm.Kind != model.KindLLM || llm.ParentID != turn.ID || llm.ReasoningTokens != 4 || llm.CacheReadTokens != 2 {
		t.Fatalf("llm = %+v", llm)
	}
	if toolStart.ID != "c1" || toolEnd.ID != "c1" || toolEnd.Error != "boom" || toolStart.ParentID != turn.ID {
		t.Fatalf("tools = %+v %+v", toolStart, toolEnd)
	}
	if compaction.Kind != model.KindCompaction || compaction.ParentID != turn.ID {
		t.Fatalf("compaction = %+v", compaction)
	}
}
