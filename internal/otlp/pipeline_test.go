package otlp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/atharvamhaske/flowtel/internal/audit"
	"github.com/atharvamhaske/flowtel/internal/otlp"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

func TestPipelineExportsTracesAndLogs(t *testing.T) {
	var mutex sync.Mutex
	paths := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		paths = append(paths, r.URL.Path)
		mutex.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	pipeline, err := otlp.New(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	event := model.Event{
		ID: "s1", SessionID: "s1", Harness: "pi", Profile: model.ProfileBoth,
		Kind: model.KindSession, Name: "flowtel.session",
	}
	record, err := audit.New(event)
	if err != nil {
		t.Fatalf("audit.New() error = %v", err)
	}
	if err := pipeline.Record(context.Background(), []model.Event{event}, []audit.Record{record}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if err := pipeline.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	mutex.Lock()
	defer mutex.Unlock()
	seen := map[string]bool{}
	for _, path := range paths {
		seen[path] = true
	}
	if !seen["/v1/traces"] || !seen["/v1/logs"] {
		t.Fatalf("export paths = %v, want traces and logs", paths)
	}
}
