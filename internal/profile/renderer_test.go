package profile_test

import (
	"testing"

	"github.com/atharvamhaske/flowtel/internal/profile"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

func TestRendererRender(t *testing.T) {
	event := model.Event{
		ID: "llm-1", SessionID: "session-1", Harness: "pi",
		Profile: model.ProfileBoth, Kind: model.KindLLM, Name: "llm",
		Model: "claude-3-7-sonnet", Provider: "anthropic",
	}

	attributes, err := profile.New().Render(event)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	checks := map[string]any{
		"flowtel.schema.version":    model.SchemaVersion,
		"flowtel.session.id":        "session-1",
		"flowtel.attribute.profile": "both",
		"openinference.span.kind":   "LLM",
		"gen_ai.operation.name":     "chat",
		"gen_ai.request.model":      "claude-3-7-sonnet",
	}
	for key, expected := range checks {
		if got := attributes[key]; got != expected {
			t.Errorf("attribute %q = %v, want %v", key, got, expected)
		}
	}
}

func TestRendererEmitsThinkingCharsButNeverZero(t *testing.T) {
	present := model.Event{
		ID: "tool-1", SessionID: "session-1", Harness: "pi",
		Profile: model.ProfileBoth, Kind: model.KindTool, Name: "tool.read",
		ToolName: "read", ThinkingChars: 24,
	}
	attributes, err := profile.New().Render(present)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got := attributes["flowtel.thinking.chars"]; got != int64(24) {
		t.Fatalf("flowtel.thinking.chars = %v, want 24", got)
	}

	absent := present
	absent.ThinkingChars = 0
	attributes, err = profile.New().Render(absent)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if _, ok := attributes["flowtel.thinking.chars"]; ok {
		t.Fatalf("flowtel.thinking.chars present with ThinkingChars=0, want absent")
	}
}

func TestRendererOpenInferenceOnlyHasSessionAndTokens(t *testing.T) {
	event := model.Event{
		ID: "llm-1", SessionID: "session-1", Harness: "pi",
		Profile: model.ProfileOpenInference, Kind: model.KindLLM, Name: "llm",
		Model: "model-1", Provider: "provider-1",
		InputTokens: 12, OutputTokens: 8, ReasoningTokens: 3, CacheReadTokens: 2,
	}
	attributes, err := profile.New().Render(event)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	checks := map[string]any{
		"session.id":                                   "session-1",
		"llm.token_count.prompt":                       int64(12),
		"llm.token_count.completion":                   int64(8),
		"llm.token_count.total":                        int64(20),
		"llm.token_count.completion_details.reasoning": int64(3),
		"llm.token_count.prompt_details.cache_read":    int64(2),
	}
	for key, expected := range checks {
		if got := attributes[key]; got != expected {
			t.Errorf("attribute %q = %v, want %v", key, got, expected)
		}
	}
}

func TestRendererEmitsCostAndCacheWriteUnderOpenInference(t *testing.T) {
	event := model.Event{
		ID: "llm-1", SessionID: "session-1", Harness: "pi",
		Profile: model.ProfileOpenInference, Kind: model.KindLLM, Name: "llm",
		Model: "model-1", Provider: "provider-1",
		CacheWriteTokens: 4,
		CostInput:        0.001, CostOutput: 0.002, CostCacheRead: 0.0001, CostCacheWrite: 0.0002, CostTotal: 0.0033,
	}
	attributes, err := profile.New().Render(event)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	checks := map[string]any{
		"llm.token_count.prompt_details.cache_write": int64(4),
		"llm.cost.prompt":                     0.001,
		"llm.cost.completion":                 0.002,
		"llm.cost.prompt_details.cache_read":  0.0001,
		"llm.cost.prompt_details.cache_write": 0.0002,
		"llm.cost.total":                      0.0033,
	}
	for key, expected := range checks {
		if got := attributes[key]; got != expected {
			t.Errorf("attribute %q = %v, want %v", key, got, expected)
		}
	}
}

func TestRendererHonorsProfile(t *testing.T) {
	event := model.Event{
		ID: "llm-1", Harness: "pi", Profile: model.ProfileOpenInference,
		Kind: model.KindLLM, Name: "llm", Model: "model-1", Provider: "provider-1",
	}
	attributes, err := profile.New().Render(event)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if _, ok := attributes["openinference.span.kind"]; !ok {
		t.Fatal("openinference attribute missing")
	}
	if _, ok := attributes["gen_ai.operation.name"]; ok {
		t.Fatal("gen_ai attribute present in openinference profile")
	}
}
