# Integration test architecture

Flowtel mirrors the useful boundaries in the Rust agent tests without copying
Braintrust types or provider credentials.

| Rust layer | Go layer | Ownership |
| --- | --- | --- |
| `server` | `internal/integration/server` | Owns one `httptest.Server` lifecycle |
| `inference` | `internal/integration/inference` | Captures OpenAI Responses and Anthropic Messages requests and returns deterministic JSON |
| `ingest` | `internal/integration/ingest` | Captures OTLP-like trace/log requests and matches ordered paths |
| `agent_process` | `internal/integration/agentprocess` | Owns context, daemon, mock servers, `os/exec` Pi, and cleanup |
| `agents` | `internal/integration/agents` | Runs a Pi fixture through the same JSON-RPC daemon wire |

The test composes these layers as:

```text
inference server + ingest server
                 |
                 v
          agent process world
                 |
                 v
           Flowtel daemon
                 |
                 v
             Pi adapter
```

Every owner has one cleanup path. The daemon uses a bounded channel, a worker
`WaitGroup`, and cancellation-aware contexts. The test world uses context
cancellation and closes its servers and daemon together. No goroutine is
left without an owner.

The Go version intentionally differs from Braintrust in three ways:

1. ingest captures OTLP traces and logs from `otlp.Sink`, not Braintrust `SpanRow`;
2. authentication and live backend selection are outside this test world;
3. fixture tests stay on the JSON-RPC pipe; `TestPiProcessCapturesEvents`
   execs real `pi --mode json` when `pi` is on `PATH` (or `FLOWTEL_PI`).
