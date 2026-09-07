package pi_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/atharvamhaske/flowtel/internal/adapter/pi"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

func TestParserRealShapeFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "pi", "session.jsonl")
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = file.Close() }()
	result, err := pi.NewParser(model.ProfileBoth, false).Parse(context.Background(), file)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(result.Events) != 4 {
		t.Fatalf("event count = %d, want 4", len(result.Events))
	}
	if result.Events[3].ParentID != "tool-1" || result.Events[3].PermissionDecision != "allow" {
		t.Fatalf("permission event = %+v", result.Events[3])
	}
}
