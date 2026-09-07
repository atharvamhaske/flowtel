package trace

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otel/attribute"

	"github.com/atharvamhaske/flowtel/internal/profile"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

type Recorder struct {
	tracer   trace.Tracer
	renderer profile.Renderer
}

func New(tracer trace.Tracer) Recorder {
	return Recorder{tracer: tracer, renderer: profile.New()}
}

func (r Recorder) Start(ctx context.Context, event model.Event) (context.Context, trace.Span, error) {
	if r.tracer == nil {
		return ctx, nil, fmt.Errorf("start span: tracer is nil")
	}
	attributes, err := r.renderer.Render(event)
	if err != nil {
		return ctx, nil, fmt.Errorf("render span attributes: %w", err)
	}
	options := make([]trace.SpanStartOption, 0, 1)
	if !event.Start.IsZero() {
		options = append(options, trace.WithTimestamp(event.Start))
	}
	spanContext, span := r.tracer.Start(ctx, event.Name, options...)
	span.SetAttributes(toAttributes(attributes)...)
	if event.Error != "" {
		span.SetStatus(codes.Error, event.Error)
	}
	return spanContext, span, nil
}

func toAttributes(values map[string]any) []attribute.KeyValue {
	attributes := make([]attribute.KeyValue, 0, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case string:
			attributes = append(attributes, attribute.String(key, typed))
		case int64:
			attributes = append(attributes, attribute.Int64(key, typed))
		}
	}
	return attributes
}
