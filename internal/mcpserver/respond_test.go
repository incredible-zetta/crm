package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestErrShape(t *testing.T) {
	res := Err("bad_stage", "invalid stage")

	if res == nil {
		t.Fatal("expected non-nil response")
	}
	if !res.IsError {
		t.Error("expected IsError to be true")
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected 1 content element, got %d", len(res.Content))
	}

	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected content to be *mcp.TextContent, got %T", res.Content[0])
	}

	var env ErrEnvelope
	if err := json.Unmarshal([]byte(tc.Text), &env); err != nil {
		t.Fatalf("failed to unmarshal text content: %v", err)
	}

	if env.Error != "bad_stage" {
		t.Errorf("expected Error %q, got %q", "bad_stage", env.Error)
	}
	if env.Msg != "invalid stage" {
		t.Errorf("expected Msg %q, got %q", "invalid stage", env.Msg)
	}
}

func TestTextResultCompact(t *testing.T) {
	input := map[string]any{"a": float64(1), "b": "x"}
	res, err := TextResult(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res == nil {
		t.Fatal("expected non-nil response")
	}
	if res.IsError {
		t.Error("expected IsError to be false")
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected 1 content element, got %d", len(res.Content))
	}

	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected content to be *mcp.TextContent, got %T", res.Content[0])
	}

	// Verify it's compact JSON (no extra whitespace or newlines)
	text := tc.Text
	if strings.Contains(text, "\n") || strings.Contains(text, " ") {
		t.Errorf("expected compact JSON, but found formatting in: %q", text)
	}

	var output map[string]any
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if output["a"] != input["a"] || output["b"] != input["b"] {
		t.Errorf("expected unmarshaled content to match input, got %v", output)
	}
}
