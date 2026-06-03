package mcpserver

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ErrEnvelope is the JSON shape: {"error":"code","msg":"short"}.
type ErrEnvelope struct {
	Error string `json:"error"`
	Msg   string `json:"msg"`
}

// Err builds a tool error result with a terse shape {error,msg}, returned as JSON text content.
// Use as the *mcp.CallToolResult return alongside a zero Out value and nil Go error
// (so the MCP layer returns a normal result the agent can read, not a transport error).
func Err(code, msg string) *mcp.CallToolResult {
	env := ErrEnvelope{
		Error: code,
		Msg:   msg,
	}
	b, err := json.Marshal(env)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: `{"error":"internal_error","msg":"failed to marshal error"}`}},
			IsError: true,
		}
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		IsError: true,
	}
}

// TextResult builds a CallToolResult whose Content is a single TextContent with compact JSON of v.
func TextResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil
}
