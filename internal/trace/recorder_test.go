package trace_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	flowtrace "github.com/atharvamhaske/flowtel/internal/trace"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

func TestRecorderStart(t *testing.T) {
	provider := trace.NewTracerProvider()
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	recorder := flowtrace.New(provider.Tracer("flowtel/test"))
	event := model.Event{
		ID: "session-1", SessionID: "session-1", Harness: "pi", Profile: model.ProfileBoth,
		Kind: model.KindSession, Name: "flowtel.session",
	}
	ctx, span, err := recorder.Start(context.Background(), event)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if ctx == nil || span == nil {
		t.Fatal("Start() returned nil context or span")
	}
	span.End()
}

func TestRecorderStartRecordsExceptionOnError(t *testing.T) {
	recorderExporter := tracetest.NewSpanRecorder()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(recorderExporter))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	recorder := flowtrace.New(provider.Tracer("flowtel/test"))
	event := model.Event{
		ID: "tool-1", SessionID: "session-1", Harness: "pi", Profile: model.ProfileBoth,
		Kind: model.KindTool, Name: "flowtel.tool", ToolName: "bash",
		Error: "exit status 1",
	}
	_, span, err := recorder.Start(context.Background(), event)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	span.End()

	spans := recorderExporter.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	got := spans[0]
	if got.Status().Code != codes.Error || got.Status().Description != "exit status 1" {
		t.Fatalf("status = %+v, want error status with the event's message", got.Status())
	}
	events := got.Events()
	if len(events) != 1 || events[0].Name != "exception" {
		t.Fatalf("span events = %+v, want one \"exception\" event", events)
	}
	var gotType, gotMessage string
	for _, attr := range events[0].Attributes {
		switch attr.Key {
		case "exception.type":
			gotType = attr.Value.AsString()
		case "exception.message":
			gotMessage = attr.Value.AsString()
		}
	}
	if gotType != "tool_error" {
		t.Fatalf("exception.type = %q, want %q", gotType, "tool_error")
	}
	if gotMessage != "exit status 1" {
		t.Fatalf("exception.message = %q, want %q", gotMessage, "exit status 1")
	}
}
