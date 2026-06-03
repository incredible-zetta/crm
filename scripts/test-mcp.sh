#!/usr/bin/env bash
# Smoke-test the CRM MCP server over Streamable HTTP.
#
# Usage:
#   MCP_URL=http://localhost:8080/mcp MCP_API_KEY=yourkey ./scripts/test-mcp.sh
#
# Defaults: MCP_URL=http://localhost:8080/mcp, MCP_API_KEY=testkey123
#
# Performs: initialize -> notifications/initialized -> tools/list -> tools/call health_check
set -euo pipefail

MCP_URL="${MCP_URL:-http://localhost:8080/mcp}"
MCP_API_KEY="${MCP_API_KEY:-testkey123}"
ACCEPT='application/json, text/event-stream'

hdr_auth="Authorization: Bearer ${MCP_API_KEY}"
hdr_ct='Content-Type: application/json'
hdr_acc="Accept: ${ACCEPT}"

echo "==> Target: ${MCP_URL}"

# 1) initialize (capture session id from response headers)
tmp_hdrs="$(mktemp)"
init_body='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"1.0.0"}}}'
echo "==> initialize"
curl -sS -D "${tmp_hdrs}" -X POST "${MCP_URL}" \
  -H "${hdr_auth}" -H "${hdr_ct}" -H "${hdr_acc}" \
  -d "${init_body}" | sed 's/^data: //; /^event:/d; /^$/d'

SID="$(grep -i '^mcp-session-id:' "${tmp_hdrs}" | awk '{print $2}' | tr -d '\r' || true)"
rm -f "${tmp_hdrs}"
if [ -z "${SID}" ]; then
  echo "WARNING: no Mcp-Session-Id header returned; subsequent calls may fail." >&2
fi
hdr_sid="Mcp-Session-Id: ${SID}"

# 2) notifications/initialized (no response expected)
curl -sS -o /dev/null -X POST "${MCP_URL}" \
  -H "${hdr_auth}" -H "${hdr_ct}" -H "${hdr_acc}" -H "${hdr_sid}" \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}'

# 3) tools/list
echo "==> tools/list"
curl -sS -X POST "${MCP_URL}" \
  -H "${hdr_auth}" -H "${hdr_ct}" -H "${hdr_acc}" -H "${hdr_sid}" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
  | sed 's/^data: //; /^event:/d; /^$/d'

# 4) tools/call health_check
echo "==> tools/call health_check"
curl -sS -X POST "${MCP_URL}" \
  -H "${hdr_auth}" -H "${hdr_ct}" -H "${hdr_acc}" -H "${hdr_sid}" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"health_check","arguments":{}}}' \
  | sed 's/^data: //; /^event:/d; /^$/d'

echo "==> done"
