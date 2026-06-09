#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.e2e.yml}"
MCP_URL="${MCP_URL:-http://localhost:8080/mcp}"
MCP_API_KEY="${MCP_API_KEY:-e2e-test-key}"
ACCEPT='application/json, text/event-stream'

cleanup() {
  docker compose -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

parse_tool_json() {
  python3 -c '
import json, sys
raw = sys.stdin.read()
lines = [line[6:] for line in raw.splitlines() if line.startswith("data: ")]
payload = "".join(lines) if lines else raw
data = json.loads(payload)
text = data["result"]["content"][0]["text"]
print(json.dumps(json.loads(text)))
'
}

mcp_init() {
  local tmp_hdrs
  tmp_hdrs="$(mktemp)"
  curl -sS -D "$tmp_hdrs" -X POST "$MCP_URL" \
    -H "Authorization: Bearer $MCP_API_KEY" \
    -H "Content-Type: application/json" \
    -H "Accept: $ACCEPT" \
    -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"e2e","version":"1.0.0"}}}' \
    >/dev/null
  MCP_SESSION_ID="$(grep -i '^mcp-session-id:' "$tmp_hdrs" | awk '{print $2}' | tr -d '\r' || true)"
  rm -f "$tmp_hdrs"
  curl -sS -o /dev/null -X POST "$MCP_URL" \
    -H "Authorization: Bearer $MCP_API_KEY" \
    -H "Content-Type: application/json" \
    -H "Accept: $ACCEPT" \
    -H "Mcp-Session-Id: $MCP_SESSION_ID" \
    -d '{"jsonrpc":"2.0","method":"notifications/initialized"}'
}

mcp_call() {
  local tool="$1"
  local args="${2:-{}}"
  local raw
  raw="$(curl -sS -X POST "$MCP_URL" \
    -H "Authorization: Bearer $MCP_API_KEY" \
    -H "Content-Type: application/json" \
    -H "Accept: $ACCEPT" \
    -H "Mcp-Session-Id: $MCP_SESSION_ID" \
    -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"$tool\",\"arguments\":$args}}")"
  echo "$raw" | parse_tool_json
}

echo "==> Building and starting e2e stack"
docker compose -f "$COMPOSE_FILE" up -d --build --wait

echo "==> Health check"
for i in $(seq 1 30); do
  if curl -fsS http://localhost:8080/healthz >/dev/null; then
    break
  fi
  sleep 2
  if [[ "$i" -eq 30 ]]; then
    echo "health check failed" >&2
    docker compose -f "$COMPOSE_FILE" logs crm
    exit 1
  fi
done

mcp_init

echo "==> Create template"
TEMPLATE_JSON="$(mcp_call template_create '{"name":"e2e-template","subject":"Hello","body_text":"Hi {{first_name}}"}')"
TEMPLATE_ID="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <<<"$TEMPLATE_JSON")"

echo "==> Create contacts"
for i in 1 2 3; do
  mcp_call contact_create "{\"email\":\"lead${i}@example.com\",\"first_name\":\"Lead${i}\",\"tags\":[\"segment_b2b\"],\"stage\":\"new\"}" >/dev/null
done

echo "==> Create scheduled campaign (due in 2s)"
RUN_AT="$(date -u -d '+2 seconds' +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -v+2S +%Y-%m-%dT%H:%M:%SZ)"
CAMPAIGN_JSON="$(mcp_call campaign_create "{\"name\":\"e2e-scheduled\",\"template_id\":$TEMPLATE_ID,\"segment\":{\"tag\":\"segment_b2b\"},\"scheduled_at\":\"$RUN_AT\"}")"
CAMPAIGN_ID="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <<<"$CAMPAIGN_JSON")"

echo "==> Verify scheduled task was created"
TASK_JSON="$(mcp_call task_list '{"status":"pending","limit":20}')"
echo "$TASK_JSON" | python3 -c "
import json, sys
campaign_id = int('$CAMPAIGN_ID')
data = json.load(sys.stdin)
items = data.get('items', [])
campaign_tasks = [i for i in items if i.get('kind') == 'campaign' and i.get('campaign_id') == campaign_id]
if not campaign_tasks:
    raise SystemExit(f'expected pending campaign task for campaign {campaign_id}, got {items}')
print('pending campaign task ok:', campaign_tasks[0])
"

echo "==> Wait for scheduler to send campaign"
for i in $(seq 1 30); do
  STATUS_JSON="$(mcp_call campaign_get "{\"id\":$CAMPAIGN_ID}")"
  STATUS="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["status"])' <<<"$STATUS_JSON")"
  if [[ "$STATUS" == "sent" ]]; then
    break
  fi
  sleep 2
  if [[ "$i" -eq 30 ]]; then
    echo "campaign did not reach sent status (last: $STATUS)" >&2
    mcp_call task_list '{}' >&2 || true
    docker compose -f "$COMPOSE_FILE" logs crm >&2
    exit 1
  fi
done

echo "==> Verify lead stages promoted to contacted"
LIST_JSON="$(mcp_call contact_list '{"tag":"segment_b2b","limit":20}')"
echo "$LIST_JSON" | python3 -c "
import json, sys
items = json.load(sys.stdin).get('items', [])
stages = {i['email']: i.get('stage') for i in items}
bad = [e for e, s in stages.items() if s != 'contacted']
if bad:
    raise SystemExit(f'expected all contacted, got {stages}')
print('all leads contacted:', list(stages.keys()))
"

echo "==> Verify campaign stats tracking metadata"
STATS_JSON="$(mcp_call campaign_stats "{\"campaign_id\":$CAMPAIGN_ID}")"
echo "$STATS_JSON" | python3 -c "
import json, sys
s = json.load(sys.stdin)
if s.get('top_links') != []:
    raise SystemExit(f\"expected empty top_links list, got {s.get('top_links')}\")
if s.get('tracking_support', {}).get('opens') != 'supported':
    raise SystemExit(f\"unexpected tracking_support: {s.get('tracking_support')}\")
print('stats ok:', {'sent': s.get('sent'), 'tracking_support': s.get('tracking_support')})
"

echo "==> E2E passed"
