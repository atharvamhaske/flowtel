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
