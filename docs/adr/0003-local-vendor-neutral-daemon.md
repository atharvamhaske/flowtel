# ADR 0003: Local vendor-neutral daemon wire

## Decision

Flowtel uses a Go daemon with a local Unix socket, newline-delimited JSON-RPC
2.0, and a durable JSONL journal. Harness adapters send opaque native payloads
inside a validated Flowtel envelope. The daemon does not know Braintrust,
Phoenix, Laminar, Greptime, or Parseable.

## Rationale

The daemon gives short-lived hooks one reliable hand-off point. Journaling and
the `accepted` response make capture independent from exporter availability.
The `Sink` interface leaves OTLP export replaceable and keeps the protocol
vendor-neutral.

## Trade-offs

- Unix sockets are used first; Windows named-pipe support is deferred until a
  Windows adapter is needed.
- The prototype journal is one JSONL file. Per-session WAL files and replay
  checkpoints are future work.
- `event.log` is durably appended before a bounded worker queue accepts it.
  The default sink is no-op. `otlp.Sink` is the OTLP delivery implementation;
  retry policy remains a later slice.
