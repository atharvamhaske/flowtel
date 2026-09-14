package otlp_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"

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

func TestPipelineExportsMetricsForLLMEvents(t *testing.T) {
	var mutex sync.Mutex
	paths := make([]string, 0, 3)
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
	start := time.Now().Add(-2 * time.Second)
	event := model.Event{
		ID: "llm-1", SessionID: "s1", Harness: "pi", Profile: model.ProfileBoth,
		Kind: model.KindLLM, Name: "flowtel.llm", Model: "claude", Provider: "anthropic",
		Start: start, End: start.Add(2 * time.Second), InputTokens: 12, OutputTokens: 8,
	}
	record, err := audit.New(event)
	if err != nil {
		t.Fatalf("audit.New() error = %v", err)
	}
	if err := pipeline.Record(context.Background(), []model.Event{event}, []audit.Record{record}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	// PeriodicReader defaults to a 60s export interval; ForceFlush forces
	// an immediate collect+export so the test doesn't wait a minute.
	if err := pipeline.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() error = %v", err)
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
	if !seen["/v1/metrics"] {
		t.Fatalf("export paths = %v, want /v1/metrics", paths)
	}
}

func TestPipelineSkipsMetricsWhenDisabled(t *testing.T) {
	t.Setenv(otlp.EnvMetricsEnabled, "0")
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
		ID: "llm-1", SessionID: "s1", Harness: "pi", Profile: model.ProfileBoth,
		Kind: model.KindLLM, Name: "flowtel.llm", Model: "claude", Provider: "anthropic",
		Start: time.Now().Add(-time.Second), End: time.Now(), InputTokens: 12,
	}
	record, err := audit.New(event)
	if err != nil {
		t.Fatalf("audit.New() error = %v", err)
	}
	if err := pipeline.Record(context.Background(), []model.Event{event}, []audit.Record{record}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if err := pipeline.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() error = %v", err)
	}
	if err := pipeline.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	mutex.Lock()
	defer mutex.Unlock()
	for _, path := range paths {
		if path == "/v1/metrics" {
			t.Fatalf("export paths = %v, want no /v1/metrics when %s=0", paths, otlp.EnvMetricsEnabled)
		}
	}
}

func TestPipelineRecordsActualEventEndTimestamp(t *testing.T) {
	var mutex sync.Mutex
	var traceBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/traces" {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			mutex.Lock()
			traceBody = body
			mutex.Unlock()
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	pipeline, err := otlp.New(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	// A start/end far in the past is exactly the batch-ingest shape this
	// guards: replaying an old session file, not a live event happening
	// "now". If span.End() ever goes back to ignoring event.End, this
	// span's duration would balloon to (time.Now() - start) instead of
	// the real 5s.
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Second)
	event := model.Event{
		ID: "s1", SessionID: "s1", Harness: "pi", Profile: model.ProfileBoth,
		Kind: model.KindSession, Name: "flowtel.session", Start: start, End: end,
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
	body := traceBody
	mutex.Unlock()
	var request coltracepb.ExportTraceServiceRequest
	if err := proto.Unmarshal(body, &request); err != nil {
		t.Fatalf("unmarshal trace export request: %v", err)
	}
	if len(request.ResourceSpans) == 0 || len(request.ResourceSpans[0].ScopeSpans) == 0 || len(request.ResourceSpans[0].ScopeSpans[0].Spans) == 0 {
		t.Fatal("no spans in export request")
	}
	span := request.ResourceSpans[0].ScopeSpans[0].Spans[0]
	gotEnd := time.Unix(0, int64(span.EndTimeUnixNano)).UTC()
	if !gotEnd.Equal(end) {
		t.Fatalf("span end = %v, want %v (event.End, not time.Now())", gotEnd, end)
	}
}
