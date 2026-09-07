// Package otlp wires Flowtel events to official OpenTelemetry exporters.
package otlp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/atharvamhaske/flowtel/internal/audit"
	flowtrace "github.com/atharvamhaske/flowtel/internal/trace"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

type Pipeline struct {
	traces   *trace.TracerProvider
	logs     *log.LoggerProvider
	record   flowtrace.Recorder
	logger   otellog.Logger
	mu       sync.Mutex
	contexts map[string]context.Context
}

func New(ctx context.Context, endpoint string) (*Pipeline, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, fmt.Errorf("create otlp pipeline: endpoint is empty")
	}
	traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpointURL(endpoint, "/v1/traces")))
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}
	logExporter, err := otlploghttp.New(ctx, otlploghttp.WithEndpointURL(endpointURL(endpoint, "/v1/logs")))
	if err != nil {
		_ = traceExporter.Shutdown(ctx)
		return nil, fmt.Errorf("create log exporter: %w", err)
	}
	traces := trace.NewTracerProvider(trace.WithBatcher(traceExporter))
	logs := log.NewLoggerProvider(log.WithProcessor(log.NewBatchProcessor(logExporter)))
	return &Pipeline{
		traces:   traces,
		logs:     logs,
		record:   flowtrace.New(traces.Tracer("github.com/atharvamhaske/flowtel")),
		logger:   logs.Logger("github.com/atharvamhaske/flowtel"),
		contexts: make(map[string]context.Context),
	}, nil
}

func (p *Pipeline) Record(ctx context.Context, events []model.Event, records []audit.Record) error {
	if p == nil || p.traces == nil || p.logs == nil {
		return fmt.Errorf("record otlp data: pipeline is nil")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.contexts == nil {
		p.contexts = make(map[string]context.Context)
	}
	for _, event := range events {
		parent := ctx
		if event.ParentID != "" {
			if parentContext, ok := p.contexts[event.ParentID]; ok {
				parent = parentContext
			}
		}
		_, span, err := p.record.Start(parent, event)
		if err != nil {
			return fmt.Errorf("record event %q: %w", event.ID, err)
		}
		p.contexts[event.ID] = oteltrace.ContextWithSpan(context.Background(), span)
		span.End()
	}
	for _, record := range records {
		var otelRecord otellog.Record
		otelRecord.SetEventName(record.Name)
		if record.Body != "" {
			otelRecord.SetBody(otellog.StringValue(record.Body))
		}
		for key, value := range record.Attributes {
			switch typed := value.(type) {
			case string:
				otelRecord.AddAttributes(otellog.String(key, typed))
			case int64:
				otelRecord.AddAttributes(otellog.Int64(key, typed))
			}
		}
		p.logger.Emit(ctx, otelRecord)
	}
	return nil
}

func (p *Pipeline) ForceFlush(ctx context.Context) error {
	if p == nil || p.traces == nil || p.logs == nil {
		return fmt.Errorf("flush otlp pipeline: pipeline is nil")
	}
	return errors.Join(p.traces.ForceFlush(ctx), p.logs.ForceFlush(ctx))
}

func (p *Pipeline) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	traceErr := p.traces.Shutdown(ctx)
	logErr := p.logs.Shutdown(ctx)
	if traceErr != nil || logErr != nil {
		return fmt.Errorf("shutdown otlp pipeline: %w", errors.Join(traceErr, logErr))
	}
	return nil
}

func endpointURL(raw, path string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimRight(raw, "/") + path
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	return parsed.String()
}
