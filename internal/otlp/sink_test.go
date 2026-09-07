package otlp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/atharvamhaske/flowtel/internal/daemon"
	"github.com/atharvamhaske/flowtel/internal/otlp"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

func TestSinkExportsEnvelope(t *testing.T) {
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
	defer func() { _ = pipeline.Shutdown(context.Background()) }()

	event := model.Event{
		ID: "s1", SessionID: "s1", Harness: "pi", Profile: model.ProfileBoth,
		Kind: model.KindSession, Name: "flowtel.session",
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	if err := otlp.NewSink(pipeline).Accept(context.Background(), daemon.Envelope{
		Source: "pi", SessionID: "s1", Event: event.Name, Payload: payload,
	}); err != nil {
		t.Fatalf("Accept() error = %v", err)
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

func TestSinkRejectsOpaquePayload(t *testing.T) {
	pipeline, err := otlp.New(context.Background(), "http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer func() { _ = pipeline.Shutdown(context.Background()) }()
	err = otlp.NewSink(pipeline).Accept(context.Background(), daemon.Envelope{
		Source: "pi", SessionID: "s1", Event: "tool", Payload: []byte(`{"kind":"tool"}`),
	})
	if err == nil {
		t.Fatal("Accept() error = nil, want invalid event")
	}
}
