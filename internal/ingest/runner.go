package ingest

import (
	"context"
	"fmt"
	"io"

	"github.com/atharvamhaske/flowtel/internal/adapter/pi"
	"github.com/atharvamhaske/flowtel/internal/audit"
	"github.com/atharvamhaske/flowtel/internal/config"
	"github.com/atharvamhaske/flowtel/internal/otlp"
)

func Run(ctx context.Context, input io.Reader, cfg config.Config, bestEffort bool) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate ingest configuration: %w", err)
	}
	if cfg.OTLPEndpoint == "" {
		return fmt.Errorf("validate ingest configuration: otlp endpoint is empty")
	}
	result, err := pi.NewParser(cfg.Profile, bestEffort).Parse(ctx, input)
	if err != nil {
		return err
	}
	records := append([]audit.Record(nil), result.Audit...)
	for _, event := range result.Events {
		record, err := audit.New(event)
		if err != nil {
			return fmt.Errorf("create audit record for %q: %w", event.ID, err)
		}
		records = append(records, record)
	}
	pipeline, err := otlp.New(ctx, cfg.OTLPEndpoint)
	if err != nil {
		return err
	}
	defer func() { _ = pipeline.Shutdown(context.Background()) }()
	if err := pipeline.Record(ctx, result.Events, records); err != nil {
		return fmt.Errorf("export ingest result: %w", err)
	}
	return nil
}
