# ADR 0001: Go Module And OpenTelemetry SDK

Status: accepted

## Decision

Flowtel uses the module path `github.com/atharvamhaske/flowtel` and the official
OpenTelemetry Go API and SDK packages. The first implementation is a library
and CLI workspace with these boundaries:

- `pkg/model` contains the public vendor-neutral event model.
- `internal/profile` renders the selected OpenInference and GenAI attribute
  profile.
- `internal/trace` owns span lifecycle and exporter wiring.
- `internal/audit` owns bounded log records.
- `internal/adapter/pi` owns Pi-specific parsing.
- `cmd/flowtel` is the later CLI entry point.

## Trade-off

The official OpenTelemetry SDK is a required dependency instead of a custom
wire model. This keeps Flowtel compatible with OTLP and the Go ecosystem, but
it makes the initial module depend on upstream versioning. Provider SDKs are
not included because the spec places Flowtel at the harness layer, after the
provider call boundary.
