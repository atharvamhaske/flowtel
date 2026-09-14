#!/usr/bin/env bash
# Exercises the real CLI chain (daemon serve -> pi run -> status) using a
# fake `pi` binary, so it needs no real pi install or model API key.
# Mirrors braintrust-coding-agent-plugins/scripts/test-hook-forwarders.sh:
# fake the external binary, run the real wrapper against it, assert on
# what actually happened.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
daemon_pid=""

cleanup() {
  if [[ -n "$daemon_pid" ]]; then
    kill "$daemon_pid" 2>/dev/null || true
    wait "$daemon_pid" 2>/dev/null || true
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT

echo "smoke: building flowctl"
go build -o "$tmp/flowctl" "$root/cmd/flowtel"

echo "smoke: writing fake pi"
mkdir -p "$tmp/sessions"
cat > "$tmp/fake-pi" <<EOF
#!/usr/bin/env bash
cp "$root/testdata/pi/session.jsonl" "$tmp/sessions/\$(date +%s%N).jsonl"
exit 0
EOF
chmod +x "$tmp/fake-pi"

echo "smoke: starting daemon"
"$tmp/flowctl" daemon serve --socket "$tmp/d.sock" --data-dir "$tmp/data" &
daemon_pid=$!

deadline=$((SECONDS + 5))
until [[ -S "$tmp/d.sock" ]]; do
  if [[ $SECONDS -ge $deadline ]]; then
    echo "smoke: FAILED - daemon socket never appeared" >&2
    exit 1
  fi
  sleep 0.05
done

echo "smoke: running flowctl pi run against the fake binary"
FLOWTEL_PI="$tmp/fake-pi" \
  FLOWTEL_HARNESS=pi \
  FLOWTEL_ATTRIBUTE_PROFILE=both \
  PI_CODING_AGENT_SESSION_DIR="$tmp/sessions" \
  "$tmp/flowctl" pi run --socket "$tmp/d.sock" -- ignored-args

status="$("$tmp/flowctl" status --socket "$tmp/d.sock")"
echo "smoke: status = $status"

events_stored="$(echo "$status" | sed -n 's/.*events_stored=\([0-9]*\).*/\1/p')"
if [[ -z "$events_stored" || "$events_stored" -lt 1 ]]; then
  echo "smoke: FAILED - expected events_stored >= 1, got '$events_stored'" >&2
  exit 1
fi

echo "smoke: e2e OK"
