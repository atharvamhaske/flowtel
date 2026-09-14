package otlp_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"gopkg.in/dnaeon/go-vcr.v4/pkg/cassette"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"

	"github.com/atharvamhaske/flowtel/internal/audit"
	"github.com/atharvamhaske/flowtel/internal/otlp"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

// EnvVCRMode selects cassette-based compatibility test behavior:
//   - "record": make real HTTP calls against the real backend, using real
//     credentials from the environment, and save them to the cassette.
//   - unset or anything else: replay only, from an existing cassette. Never
//     falls through to a live call — a missing cassette is a skip (the
//     backend hasn't been recorded yet), not a silent network attempt.
const EnvVCRMode = "FLOWTEL_VCR_MODE"

// TestBraintrustCompat proves flowtel's own OTLP trace exporter is accepted
// by Braintrust's real API, talking to it directly — bypassing the
// Collector entirely, per docs/adr/0004. This is SPEC.md §10's Braintrust
// compatibility target, automated.
func TestBraintrustCompat(t *testing.T) {
	recording := strings.EqualFold(os.Getenv(EnvVCRMode), "record")

	// Credentials are checked before the recorder is ever constructed:
	// once recorder.New() succeeds in record mode, t.Cleanup's Stop()
	// persists whatever's in the (possibly still-empty) cassette even if
	// the test fails later — so a missing credential must never let a
	// recorder come into existence in the first place, or it leaves a
	// stray empty cassette file on disk.
	endpoint, apiKey, projectID := "https://api.braintrust.dev/otel", "replay", "replay"
	if recording {
		endpoint = requireEnv(t, "BRAINTRUST_OTLP_ENDPOINT")
		apiKey = requireEnv(t, "BRAINTRUST_API_KEY")
		projectID = requireEnv(t, "BRAINTRUST_PROJECT_ID")
	}

	mode := recorder.ModeReplayOnly
	if recording {
		mode = recorder.ModeRecordOnly
	}

	rec, err := recorder.New("testdata/cassettes/braintrust",
		recorder.WithMode(mode),
		recorder.WithHook(redactAuthHeaders, recorder.BeforeSaveHook),
	)
	if errors.Is(err, cassette.ErrCassetteNotFound) {
		t.Skipf("no cassette recorded yet for braintrust — run with %s=record and real BRAINTRUST_* credentials to create internal/otlp/testdata/cassettes/braintrust.yaml (path is relative to this package, per Go's testdata convention, not the repo root)", EnvVCRMode)
	}
	if err != nil {
		t.Fatalf("recorder.New() error = %v", err)
	}
	t.Cleanup(func() { _ = rec.Stop() })

	// Braintrust doesn't ingest OTLP metrics (confirmed against its own
	// docs); disabling here keeps this test scoped to the trace signal
	// SPEC.md's compatibility target actually names.
	t.Setenv(otlp.EnvMetricsEnabled, "0")

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
		"x-bt-parent":   "project_id:" + projectID,
	}
	pipeline, err := otlp.NewWithClient(context.Background(), endpoint, rec.GetDefaultClient(), headers)
	if err != nil {
		t.Fatalf("NewWithClient() error = %v", err)
	}

	event := model.Event{
		ID: "compat-llm-1", SessionID: "compat-session-1", Harness: "pi",
		Profile: model.ProfileBoth, Kind: model.KindLLM, Name: "flowtel.llm",
		Model: "claude-3-7-sonnet", Provider: "anthropic",
		Start: time.Unix(1735689600, 0), End: time.Unix(1735689602, 0),
		InputTokens: 12, OutputTokens: 8,
	}
	record, err := audit.New(event)
	if err != nil {
		t.Fatalf("audit.New() error = %v", err)
	}
	if err := pipeline.Record(context.Background(), []model.Event{event}, []audit.Record{record}); err != nil {
		t.Fatalf("Record() error = %v — Braintrust rejected flowtel's OTLP payload", err)
	}
	if err := pipeline.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() error = %v", err)
	}
	if err := pipeline.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func requireEnv(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s=record requires %s to be set", EnvVCRMode, key)
	}
	return value
}

// redactAuthHeaders strips credential-bearing headers before a cassette
// ever touches disk, so it's structurally impossible to commit a live
// credential — not a manual review step.
func redactAuthHeaders(i *cassette.Interaction) error {
	i.Request.Headers.Del("Authorization")
	i.Request.Headers.Del("X-Bt-Parent")
	return nil
}
