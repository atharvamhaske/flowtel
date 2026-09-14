# ADR 0004: VCR/cassette compatibility tests

## Decision

`SPEC.md` §10 names five backend compatibility targets (Phoenix, Braintrust,
Laminar, Greptime, Parseable), each with a named acceptance test. None were
automated before this ADR. Flowtel now records real HTTP interactions
against each real backend once, using `gopkg.in/dnaeon/go-vcr.v4`, and
replays them in every subsequent test run — no network, no real credentials
needed in CI.

Cassettes record flowtel's own OTLP exporters (`internal/otlp`) talking
**directly** to the real backend, bypassing the Collector entirely. One
cassette per backend, stored at `internal/otlp/testdata/cassettes/<backend>
.yaml`, following Go's own `testdata/` convention (resolved relative to the
package, not the repo root).

Credentials are checked before the recorder is ever constructed: a missing
env var in record mode fails immediately, before `recorder.New()` runs, so a
failed record attempt never leaves a stray empty cassette on disk (discovered
the hard way while building this — the naive order left exactly that
artifact behind).

Mode selection is explicit, via `FLOWTEL_VCR_MODE` (`"record"` or unset for
replay-only). Replay mode never silently falls through to a live call: a
missing cassette is `t.Skip`, not a network attempt, and go-vcr's
`ModeReplayOnly` itself refuses to record new interactions if replay can't
find a match. Secret redaction is automatic and mandatory — a
`BeforeSaveHook` strips `Authorization` and `X-Bt-Parent` before a cassette
ever touches disk, not a manual review step before committing.

## Rationale

`go-vcr` wraps an `*http.Client`, which only works cleanly for HTTP calls
happening inside the Go test process. `internal/otlp/pipeline.go`'s
exporters (`otlptracehttp`, `otlploghttp`, `otlpmetrichttp`) are in-process,
so cassetting *those* directly against each real backend is tractable.

## Trade-offs

- **The Collector-included path is explicitly out of scope, deferred.** The
  Collector runs as a separate process (typically Docker); its outbound HTTP
  isn't something a Go test can wrap with `go-vcr`. That means these
  compatibility tests don't prove the Collector's own config (`transform/
  flowtel`'s redaction OTTL statements, the exporter headers/routing) is
  correct — only that flowtel's own exporter code produces a wire-correct
  payload the real backend accepts. Proving the full path would need a
  different mechanism (a recording HTTP proxy sitting in front of the
  Collector, intercepting its outbound calls) — not built yet.
- Braintrust is built first, proving the pattern with credentials already on
  hand from earlier manual testing. Phoenix, Laminar, Greptime, and Parseable
  each need their own recording session before their cassette exists; until
  then, their tests (once written) will skip, not fail.
- Cassettes are per-backend, not per-test-function — matching SPEC.md §10's
  table directly, at the cost of one shared cassette covering whatever a
  given compatibility test happens to send, rather than fine-grained
  per-scenario recordings.
