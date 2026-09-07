# Flowtel Specification

Status: planning baseline v0.1  
Module: `github.com/atharvamhaske/flowtel`

## 1. Purpose

Flowtel turns coding-harness activity into vendor-neutral OpenTelemetry data.
It records nested session, LLM, tool, and permission activity as OTLP traces,
and terminal audit events as OTLP logs. It ships no dashboard, agent runtime,
provider SDK wrapper, or proprietary vendor attributes.

This specification replaces the earlier `agentotel` draft for Flowtel work.

## 2. Goals

- Preserve parentage: session -> LLM or tool -> permission.
- Support published OpenInference and OpenTelemetry GenAI semantics on the
  same spans.
- Export OTLP without a destination-specific data model.
- Start with Pi as the first harness adapter.
- Emit small, correlated audit logs for operational backends.
- Prove traces in Phoenix and Braintrust; prove logs in Greptime and
  Parseable. Laminar is a trace-compatibility target. Bifrost is optional.

## 3. Non-goals

- No Flowtel UI, storage, SaaS, agent runtime, planner, or eBPF collector.
- No OpenAI, Anthropic, or other model SDK dependency in the Flowtel core.
- No universal parser. Each harness gets its own tested adapter.
- No `braintrust.*`, `laminar.*`, `greptime.*`, or `parseable.*` attributes.
- No raw prompts, completions, tool output, or per-token logs by default.

## 4. Architecture

```text
Pi JSONL + Pi permission extension
                 |
                 v
        Flowtel adapter and IR
                 |
       +---------+---------+
       |                   |
       v                   v
 OTLP spans            OTLP audit logs
       |                   |
       +------ OTel Collector ------+
              transform + batch
              |                    |
       trace backends          log backends
 Phoenix / Braintrust /      Greptime / Parseable
 Laminar
```

Flowtel owns the IR and its rendering. The Collector only applies generic
resource enrichment, redaction, transformation, batching, and fan-out.

## 5. Profiles

`both` is the default.

| Profile | Span attributes |
| --- | --- |
| `openinference` | Published OpenInference span kind and LLM/tool attributes |
| `gen_ai` | Pinned OpenTelemetry `gen_ai.*` attributes |
| `both` | Union of both sets on the same span |

All profiles include stable Flowtel attributes:

| Attribute | Required | Meaning |
| --- | --- | --- |
| `flowtel.schema.version` | yes | `0.1.0` |
| `flowtel.session.id` | yes when known | Harness session identifier |
| `flowtel.harness` | yes | `pi` for v0.1 |
| `flowtel.attribute.profile` | yes | selected profile |

## 6. Trace contract

| Logical span | Name | Parent | Required Flowtel attributes |
| --- | --- | --- | --- |
| Session | `flowtel.session` | trace root | session id, harness, schema version, profile |
| LLM | `llm` | session | model/provider when present |
| Tool | `tool.{name}` | session or LLM | tool name |
| Permission | `flowtel.permission` | related tool | decision, tool when known, source |

Permissions use `flowtel.permission.decision=allow|deny`,
`flowtel.permission.tool`, and `flowtel.permission.source`. There is no
published OpenInference or GenAI equivalent, so Flowtel keeps this namespace.

Errors use standard OpenTelemetry status and exception attributes. Timestamps
from the harness are retained. Missing times use documented source order; the
adapter must not reorder events.

## 7. Log contract

Logs use the OpenTelemetry LogRecord data model. OpenInference and GenAI are
span conventions, not Flowtel log namespaces.

One terminal audit record is emitted for each of these events:

- `flowtel.session.completed`
- `flowtel.llm.completed`
- `flowtel.tool.completed`
- `flowtel.permission.decided`
- `flowtel.parser.failed`

Required log attributes:

| Attribute | Type | Meaning |
| --- | --- | --- |
| `flowtel.schema.version` | string | `0.1.0` |
| `flowtel.event.name` | string | one event name above |
| `flowtel.harness` | string | source harness |
| `flowtel.session.id` | string | session identifier when known |
| `flowtel.attribute.profile` | string | span profile in effect |

Optional scalar attributes include `flowtel.tool.name`,
`flowtel.permission.decision`, `flowtel.parser.line`, `gen_ai.request.model`,
and token counts. Logs rely on OTLP trace and span correlation fields rather
than duplicate trace IDs as custom attributes.

The body is a bounded, redacted summary. Raw message bodies and tool payloads
require a future explicit opt-in policy.

## 8. Pi adapter v0.1

The adapter reads Pi session JSONL and a small Pi extension audit stream.

- assistant records map to LLM activity when model/usage is present;
- tool calls and results map to tool spans;
- the extension records actual permission allow/deny decisions before tool
  execution;
- JSONL branches are represented as source metadata, not flattened into false
  parent-child execution order;
- malformed input increments a parser counter and emits one bounded audit log;
  default export fails without partial output. `--best-effort` is explicit.

Claude Code, OpenCode, OMP, and other harnesses are later adapters with their
own fixtures and documented source contracts.

## 9. Collector contract

Use stock Collector components first:

```text
memory_limiter -> transform/resource -> batch -> OTLP exporters
```

The trace pipeline fans out to Phoenix, Braintrust, and Laminar. The log
pipeline fans out to Greptime and Parseable. A shared transform keeps only the
normative resource and `flowtel.*` fields, caps log bodies, and removes
disallowed payload fields before any exporter.

Do not create a Flowtel-specific Collector processor. If compatibility fixtures
show a generic missing Collector capability, document the reproduction, open a
generic upstream issue, and contribute only a generic component after review.

## 10. Compatibility targets

| Destination | Signal | Acceptance test |
| --- | --- | --- |
| Phoenix | traces | OpenInference tree and Flowtel permission span visible |
| Braintrust | traces | `gen_ai.*` and Flowtel tree visible through OTLP |
| Laminar | traces | OTLP trace retained without vendor mapping |
| Greptime | logs | scalar Flowtel fields queryable with trace correlation |
| Parseable | logs | scalar Flowtel fields retain stable types after ingestion |
| Bifrost | optional | gateway OTLP does not duplicate or break Flowtel trace proof |

## 11. Tests

- Pi JSONL plus permission fixtures -> golden trace tree for each profile.
- Audit log goldens verify names, scalar types, bounds, and correlation.
- Attribute lint verifies required profile keys.
- Mock OTLP HTTP smoke test verifies trace and log requests.
- Compatibility fixtures run against each selected backend or its documented
  local equivalent.
- No network is needed for unit tests.

## 12. Upstream-gap record

`docs/gaps.md` will cite upstream documentation and reproductions for:

- no common harness-event to OTLP adapter for Go coding agents;
- no published permission decision span convention;
- no portable, type-stable audit-log profile for agent harnesses;
- any proven Collector transformation gap.

Flowtel exists to fill those gaps without becoming a vendor SDK. Upstream
contribution is evidence-gated, generic, and separate from v0.1 delivery.

## 13. Delivery order

1. Create module, license, this specification, and fixtures.
2. Implement core IR, profile renderer, traces, and bounded audit logs.
3. Implement Pi parser and permission extension fixture.
4. Add stock Collector fan-out configuration.
5. Prove Phoenix, Braintrust, Greptime, Parseable, and Laminar compatibility.
6. Run the Bifrost integration spike.
7. Publish gap evidence; consider generic upstream contribution only if needed.
