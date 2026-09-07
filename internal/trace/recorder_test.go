package trace_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/sdk/trace"

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
