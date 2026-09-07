package audit_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/atharvamhaske/flowtel/internal/audit"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

func TestNewRecord(t *testing.T) {
	event := model.Event{
		ID: "tool-1", SessionID: "session-1", Harness: "pi", Profile: model.ProfileBoth,
		Kind: model.KindTool, Name: "tool.exec", ToolName: "shell",
	}
	record, err := audit.New(event)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if record.Name != "flowtel.tool.completed" {
		t.Fatalf("Name = %q, want flowtel.tool.completed", record.Name)
	}

	record.Body = strings.Repeat("x", audit.MaxBodyLength+10)
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(decoded.Body) != audit.MaxBodyLength {
		t.Fatalf("body length = %d, want %d", len(decoded.Body), audit.MaxBodyLength)
	}
}
