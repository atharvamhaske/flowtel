package profile

import (
	"fmt"

	"github.com/atharvamhaske/flowtel/pkg/model"
)

type Renderer struct{}

func New() Renderer {
	return Renderer{}
}

func (Renderer) Render(event model.Event) (map[string]any, error) {
	if err := event.Validate(); err != nil {
		return nil, fmt.Errorf("validate event: %w", err)
	}

	attributes := map[string]any{
		"flowtel.schema.version":    model.SchemaVersion,
		"flowtel.harness":           event.Harness,
		"flowtel.attribute.profile": string(event.Profile),
	}
	if event.SessionID != "" {
		attributes["flowtel.session.id"] = event.SessionID
	}
	if event.Profile == model.ProfileOpenInference || event.Profile == model.ProfileBoth {
		attributes["openinference.span.kind"] = openInferenceKind(event.Kind)
		if event.SessionID != "" {
			// OpenInference's own session.id (not flowtel.session.id) is
			// what Phoenix's session-grouping UI reads.
			attributes["session.id"] = event.SessionID
		}
		if event.Model != "" {
			attributes["llm.model_name"] = event.Model
		}
		if event.Provider != "" {
			attributes["llm.provider"] = event.Provider
		}
		if event.ToolName != "" {
			attributes["tool.name"] = event.ToolName
		}
		if event.InputTokens != 0 {
			attributes["llm.token_count.prompt"] = event.InputTokens
		}
		if event.OutputTokens != 0 {
			attributes["llm.token_count.completion"] = event.OutputTokens
		}
		if event.InputTokens != 0 || event.OutputTokens != 0 {
			attributes["llm.token_count.total"] = event.InputTokens + event.OutputTokens
		}
		if event.ReasoningTokens != 0 {
			attributes["llm.token_count.completion_details.reasoning"] = event.ReasoningTokens
		}
		if event.CacheReadTokens != 0 {
			attributes["llm.token_count.prompt_details.cache_read"] = event.CacheReadTokens
		}
		if event.CacheWriteTokens != 0 {
			attributes["llm.token_count.prompt_details.cache_write"] = event.CacheWriteTokens
		}
		// Cost is an OpenInference-only concept — GenAI semconv has no
		// equivalent attribute as of writing.
		if event.CostInput != 0 {
			attributes["llm.cost.prompt"] = event.CostInput
		}
		if event.CostOutput != 0 {
			attributes["llm.cost.completion"] = event.CostOutput
		}
		if event.CostCacheRead != 0 {
			attributes["llm.cost.prompt_details.cache_read"] = event.CostCacheRead
		}
		if event.CostCacheWrite != 0 {
			attributes["llm.cost.prompt_details.cache_write"] = event.CostCacheWrite
		}
		if event.CostTotal != 0 {
			attributes["llm.cost.total"] = event.CostTotal
		}
	}
	if event.Profile == model.ProfileGenAI || event.Profile == model.ProfileBoth {
		attributes["gen_ai.operation.name"] = genAIOperation(event.Kind)
		if event.Model != "" {
			attributes["gen_ai.request.model"] = event.Model
		}
		if event.Provider != "" {
			attributes["gen_ai.system"] = event.Provider
		}
		if event.ToolName != "" {
			attributes["gen_ai.tool.name"] = event.ToolName
		}
		if event.InputTokens != 0 {
			attributes["gen_ai.usage.input_tokens"] = event.InputTokens
		}
		if event.OutputTokens != 0 {
			attributes["gen_ai.usage.output_tokens"] = event.OutputTokens
		}
		if event.ReasoningTokens != 0 {
			attributes["gen_ai.usage.reasoning_tokens"] = event.ReasoningTokens
		}
		if event.CacheReadTokens != 0 {
			attributes["gen_ai.usage.cached_input_tokens"] = event.CacheReadTokens
		}
	}
	if event.Kind == model.KindPermission {
		attributes["flowtel.permission.decision"] = event.PermissionDecision
		if event.PermissionSource != "" {
			attributes["flowtel.permission.source"] = event.PermissionSource
		}
	}
	return attributes, nil
}

func openInferenceKind(kind model.Kind) string {
	switch kind {
	case model.KindSession, model.KindAgent, model.KindTurn, model.KindPermission, model.KindCompaction:
		return "CHAIN"
	case model.KindLLM:
		return "LLM"
	case model.KindTool:
		return "TOOL"
	default:
		return "UNKNOWN"
	}
}

func genAIOperation(kind model.Kind) string {
	switch kind {
	case model.KindLLM:
		return "chat"
	case model.KindTool:
		return "execute_tool"
	default:
		return string(kind)
	}
}
