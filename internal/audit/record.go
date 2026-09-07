package audit

import (
	"encoding/json"
	"fmt"

	"github.com/atharvamhaske/flowtel/internal/bounds"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

const MaxBodyLength = 512

type Record struct {
	Name       string         `json:"event_name"`
	Attributes map[string]any `json:"attributes"`
	Body       string         `json:"body,omitempty"`
}

func New(event model.Event) (Record, error) {
	if err := event.Validate(); err != nil {
		return Record{}, fmt.Errorf("validate event: %w", err)
	}
	name, ok := eventName(event.Kind)
	if !ok {
		return Record{}, fmt.Errorf("unsupported audit event kind %q", event.Kind)
	}
	attributes := map[string]any{
		"flowtel.schema.version":    model.SchemaVersion,
		"flowtel.event.name":        name,
		"flowtel.harness":           event.Harness,
		"flowtel.attribute.profile": string(event.Profile),
	}
	if event.SessionID != "" {
		attributes["flowtel.session.id"] = event.SessionID
	}
	if event.ToolName != "" {
		attributes["flowtel.tool.name"] = event.ToolName
	}
	if event.PermissionDecision != "" {
		attributes["flowtel.permission.decision"] = event.PermissionDecision
	}
	return Record{Name: name, Attributes: attributes}, nil
}

func (r Record) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name       string         `json:"event_name"`
		Attributes map[string]any `json:"attributes"`
		Body       string         `json:"body,omitempty"`
	}{r.Name, r.Attributes, bounds.String(r.Body, MaxBodyLength)})
}

func eventName(kind model.Kind) (string, bool) {
	switch kind {
	case model.KindSession:
		return "flowtel.session.completed", true
	case model.KindAgent:
		return "flowtel.agent.completed", true
	case model.KindTurn:
		return "flowtel.turn.completed", true
	case model.KindLLM:
		return "flowtel.llm.completed", true
	case model.KindTool:
		return "flowtel.tool.completed", true
	case model.KindPermission:
		return "flowtel.permission.decided", true
	case model.KindCompaction:
		return "flowtel.compaction.completed", true
	default:
		return "", false
	}
}
