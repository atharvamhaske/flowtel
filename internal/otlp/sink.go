package otlp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/atharvamhaske/flowtel/internal/audit"
	"github.com/atharvamhaske/flowtel/internal/daemon"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

// Sink turns a daemon envelope into OTLP traces and logs.
type Sink struct {
	pipeline *Pipeline
}

func NewSink(pipeline *Pipeline) *Sink {
	return &Sink{pipeline: pipeline}
}

func (s *Sink) Accept(ctx context.Context, envelope daemon.Envelope) error {
	if s == nil || s.pipeline == nil {
		return fmt.Errorf("accept envelope: otlp sink is nil")
	}
	event, err := eventFromEnvelope(envelope)
	if err != nil {
		return err
	}
	record, err := audit.New(event)
	if err != nil {
		return fmt.Errorf("create audit record: %w", err)
	}
	if err := s.pipeline.Record(ctx, []model.Event{event}, []audit.Record{record}); err != nil {
		return err
	}
	if err := s.pipeline.ForceFlush(ctx); err != nil {
		return fmt.Errorf("flush otlp export: %w", err)
	}
	return nil
}

func eventFromEnvelope(envelope daemon.Envelope) (model.Event, error) {
	var event model.Event
	if err := json.Unmarshal(envelope.Payload, &event); err != nil {
		return model.Event{}, fmt.Errorf("decode event payload: %w", err)
	}
	if event.SessionID == "" {
		event.SessionID = envelope.SessionID
	}
	if err := event.Validate(); err != nil {
		return model.Event{}, fmt.Errorf("validate event payload: %w", err)
	}
	return event, nil
}
