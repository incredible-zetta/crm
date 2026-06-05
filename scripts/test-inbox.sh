#!/usr/bin/env bash
set -euo pipefail

BASE_URL=${BASE_URL:-http://localhost:8080}
MCP_API_KEY=${MCP_API_KEY:?set MCP_API_KEY}

call_tool() {
  local name=$1
  local args=${2:-{}}
  curl -sS "$BASE_URL/mcp" \
    -H "Authorization: Bearer $MCP_API_KEY" \
    -H 'Content-Type: application/json' \
    -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"$name\",\"arguments\":$args}}" | jq .
}

echo "== inbox_sync =="
call_tool inbox_sync '{"limit":10}'

echo "== inbox_list =="
call_tool inbox_list '{"limit":5}'

echo "== Optional examples =="
echo "call_tool inbox_get '{\"id\":55}'"
echo "call_tool inbox_mark_read '{\"id\":55,\"read\":true}'"
echo "call_tool inbox_reply '{\"id\":55,\"body_text\":\"Halo, siap.\"}'"
echo "call_tool inbox_delete '{\"id\":55}'"
