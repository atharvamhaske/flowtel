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
		if event.Model != "" {
			attributes["llm.model_name"] = event.Model
		}
		if event.Provider != "" {
			attributes["llm.provider"] = event.Provider
		}
		if event.ToolName != "" {
			attributes["tool.name"] = event.ToolName
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
	case model.KindSession:
		return "CHAIN"
	case model.KindLLM:
		return "LLM"
	case model.KindTool:
		return "TOOL"
	case model.KindPermission:
		return "CHAIN"
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
