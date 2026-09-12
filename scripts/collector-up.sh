#!/usr/bin/env bash
# Runs the OTel Collector against a flowtel config, sourcing per-backend
# credentials from configs/collector/.env (see .env.example) instead of
# requiring them exported by hand every session, and colorizes its logs.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
env_file="$root/configs/collector/.env"
if [[ -f "$env_file" ]]; then
	set -a
	# shellcheck source=/dev/null
	source "$env_file"
	set +a
fi

config="${FLOWTEL_COLLECTOR_CONFIG:-$root/configs/collector/flowtel.braintrust-only.yaml}"

docker run --rm -p 4317:4317 -p 4318:4318 \
	-v "$config:/etc/otelcol-contrib/config.yaml" \
	-e BRAINTRUST_OTLP_ENDPOINT -e BRAINTRUST_API_KEY -e BRAINTRUST_PROJECT_ID \
	-e PHOENIX_OTLP_ENDPOINT -e LAMINAR_OTLP_ENDPOINT \
	-e GREPTIME_OTLP_ENDPOINT -e PARSEABLE_OTLP_ENDPOINT \
	otel/opentelemetry-collector-contrib:latest 2>&1 | awk -f "$root/scripts/colorize-logs.awk"
