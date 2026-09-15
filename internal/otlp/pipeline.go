// Package otlp wires Flowtel events to official OpenTelemetry exporters.
package otlp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otellog "go.opentelemetry.io/otel/log"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/atharvamhaske/flowtel/internal/audit"
	flowtrace "github.com/atharvamhaske/flowtel/internal/trace"
	"github.com/atharvamhaske/flowtel/pkg/model"
)

// EnvMetricsEnabled toggles OTLP metrics export. Default on; set to "0" to
// skip it entirely. Not every configured backend accepts OTLP metrics
// (e.g. Braintrust, Laminar don't as of writing) — without this, a user
// wired only to those gets a periodic export error every ~60s forever.
const EnvMetricsEnabled = "FLOWTEL_METRICS"

// EnvEnvironment and EnvRelease stamp deployment.environment.name and
// service.version onto every span, log record, and metric this pipeline
// exports, so a backend can filter/group by deploy stage or build. Both are
// optional — unset means the resource just carries service.name.
const (
	EnvEnvironment = "FLOWTEL_ENVIRONMENT"
	EnvRelease     = "FLOWTEL_RELEASE"
)

type Pipeline struct {
	traces            *trace.TracerProvider
	logs              *log.LoggerProvider
	metrics           *metric.MeterProvider // nil when metrics export is disabled
	tokenUsage        otelmetric.Int64Histogram
	operationDuration otelmetric.Float64Histogram
	record            flowtrace.Recorder
	logger            otellog.Logger
	mu                sync.Mutex
	contexts          map[string]context.Context
}

func New(ctx context.Context, endpoint string) (*Pipeline, error) {
	return NewWithClient(ctx, endpoint, nil, nil)
}

// NewWithClient is like New but lets a caller inject a custom *http.Client
// and extra per-request headers on the trace and log exporters. It exists
// for internal/otlp/compat_test.go, which routes through a VCR recorder
// (github.com/dnaeon/go-vcr) and carries a real backend's own auth headers
// when bypassing the Collector entirely — see docs/adr/0004 for why the
// compatibility tests talk to a real backend directly rather than through
// the Collector.
func NewWithClient(ctx context.Context, endpoint string, client *http.Client, headers map[string]string) (*Pipeline, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, fmt.Errorf("create otlp pipeline: endpoint is empty")
	}
	traceOpts := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(endpointURL(endpoint, "/v1/traces"))}
	logOpts := []otlploghttp.Option{otlploghttp.WithEndpointURL(endpointURL(endpoint, "/v1/logs"))}
	if client != nil {
		traceOpts = append(traceOpts, otlptracehttp.WithHTTPClient(client))
		logOpts = append(logOpts, otlploghttp.WithHTTPClient(client))
	}
	if len(headers) > 0 {
		traceOpts = append(traceOpts, otlptracehttp.WithHeaders(headers))
		logOpts = append(logOpts, otlploghttp.WithHeaders(headers))
	}
	traceExporter, err := otlptracehttp.New(ctx, traceOpts...)
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}
	logExporter, err := otlploghttp.New(ctx, logOpts...)
	if err != nil {
		_ = traceExporter.Shutdown(ctx)
		return nil, fmt.Errorf("create log exporter: %w", err)
	}
	res, err := buildResource()
	if err != nil {
		_ = traceExporter.Shutdown(ctx)
		_ = logExporter.Shutdown(ctx)
		return nil, fmt.Errorf("build otel resource: %w", err)
	}
	traces := trace.NewTracerProvider(trace.WithBatcher(traceExporter), trace.WithResource(res))
	logs := log.NewLoggerProvider(log.WithProcessor(log.NewBatchProcessor(logExporter)), log.WithResource(res))
	pipeline := &Pipeline{
		traces:   traces,
		logs:     logs,
		record:   flowtrace.New(traces.Tracer("github.com/atharvamhaske/flowtel")),
		logger:   logs.Logger("github.com/atharvamhaske/flowtel"),
		contexts: make(map[string]context.Context),
	}
	if metricsEnabled() {
		if err := pipeline.initMetrics(ctx, endpoint, res); err != nil {
			_ = traceExporter.Shutdown(ctx)
			_ = logExporter.Shutdown(ctx)
			return nil, err
		}
	}
	return pipeline, nil
}

func metricsEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvMetricsEnabled))) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

// buildResource merges the SDK's default resource (telemetry.sdk.*,
// process/host detection) with flowtel's own service identity, so backends
// can tell flowtel's spans apart from other instrumented services and,
// optionally, filter by deploy environment or release.
func buildResource() (*resource.Resource, error) {
	attrs := []attribute.KeyValue{semconv.ServiceName("flowtel")}
	if environment := strings.TrimSpace(os.Getenv(EnvEnvironment)); environment != "" {
		attrs = append(attrs, semconv.DeploymentEnvironmentNameKey.String(environment))
	}
	if release := strings.TrimSpace(os.Getenv(EnvRelease)); release != "" {
		attrs = append(attrs, semconv.ServiceVersion(release))
	}
	return resource.Merge(resource.Default(), resource.NewSchemaless(attrs...))
}

func (p *Pipeline) initMetrics(ctx context.Context, endpoint string, res *resource.Resource) error {
	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(endpointURL(endpoint, "/v1/metrics")))
	if err != nil {
		return fmt.Errorf("create metric exporter: %w", err)
	}
	meterProvider := metric.NewMeterProvider(metric.WithReader(metric.NewPeriodicReader(metricExporter)), metric.WithResource(res))
	meter := meterProvider.Meter("github.com/atharvamhaske/flowtel")
	tokenUsage, err := meter.Int64Histogram("gen_ai.client.token.usage", otelmetric.WithUnit("{token}"))
	if err != nil {
		_ = meterProvider.Shutdown(ctx)
		return fmt.Errorf("create token usage histogram: %w", err)
	}
	operationDuration, err := meter.Float64Histogram("gen_ai.client.operation.duration", otelmetric.WithUnit("s"))
	if err != nil {
		_ = meterProvider.Shutdown(ctx)
		return fmt.Errorf("create operation duration histogram: %w", err)
	}
	p.metrics = meterProvider
	p.tokenUsage = tokenUsage
	p.operationDuration = operationDuration
	return nil
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
		if event.End.IsZero() {
			span.End()
		} else {
			span.End(oteltrace.WithTimestamp(event.End))
		}
		p.recordMetrics(ctx, event)
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

// recordMetrics records the two standard GenAI semconv histograms for a
// completed LLM call. No-op when metrics export is disabled (p.tokenUsage
// is nil) or the event isn't a fully-timed LLM span. Attributes are
// deliberately limited to low-cardinality dimensions (model, provider,
// harness, token type) — never session.id or event.ID, which would be a
// per-session timeseries explosion in a metrics backend.
func (p *Pipeline) recordMetrics(ctx context.Context, event model.Event) {
	if p.tokenUsage == nil || p.operationDuration == nil {
		return
	}
	if event.Kind != model.KindLLM || event.Start.IsZero() || event.End.IsZero() {
		return
	}
	baseAttrs := []attribute.KeyValue{
		attribute.String("gen_ai.request.model", event.Model),
		attribute.String("gen_ai.system", event.Provider),
		attribute.String("flowtel.harness", event.Harness),
	}
	p.operationDuration.Record(ctx, event.End.Sub(event.Start).Seconds(), otelmetric.WithAttributes(baseAttrs...))
	if event.InputTokens != 0 {
		p.tokenUsage.Record(ctx, event.InputTokens, otelmetric.WithAttributes(append(baseAttrs, attribute.String("gen_ai.token.type", "input"))...))
	}
	if event.OutputTokens != 0 {
		p.tokenUsage.Record(ctx, event.OutputTokens, otelmetric.WithAttributes(append(baseAttrs, attribute.String("gen_ai.token.type", "output"))...))
	}
}

func (p *Pipeline) ForceFlush(ctx context.Context) error {
	if p == nil || p.traces == nil || p.logs == nil {
		return fmt.Errorf("flush otlp pipeline: pipeline is nil")
	}
	if p.metrics == nil {
		return errors.Join(p.traces.ForceFlush(ctx), p.logs.ForceFlush(ctx))
	}
	return errors.Join(p.traces.ForceFlush(ctx), p.logs.ForceFlush(ctx), p.metrics.ForceFlush(ctx))
}

func (p *Pipeline) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	traceErr := p.traces.Shutdown(ctx)
	logErr := p.logs.Shutdown(ctx)
	var metricErr error
	if p.metrics != nil {
		metricErr = p.metrics.Shutdown(ctx)
	}
	if traceErr != nil || logErr != nil || metricErr != nil {
		return fmt.Errorf("shutdown otlp pipeline: %w", errors.Join(traceErr, logErr, metricErr))
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
