# Upstream Gap Record

Flowtel exists because coding-agent harnesses expose useful lifecycle events,
but there is no common Go adapter that turns those events into portable OTLP.
Provider SDKs describe model calls. They do not describe the repository, shell,
file, branch, or permission work around those calls.

The second gap is semantic. OpenInference and OTel GenAI define span
conventions, but neither defines a permission-decision span or a bounded,
type-stable audit-log profile for harness activity. Flowtel keeps those fields
under `flowtel.*` and leaves backend routing to the stock Collector.

The Collector configuration in this repository is intentionally generic. A
custom processor is not part of v0.1. If a reproducible transform or size-cap
gap remains after the stock `memory_limiter`, transform, and batch processors
are tested, record the upstream issue and proposed generic component here
before contributing code.
