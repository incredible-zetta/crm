package mcpserver

import (
	"context"
	"net/http"
	"testing"

	"github.com/incredible-zetta/crm/internal/tenant"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeResolver struct {
	gotAPIKey    string
	gotSessionID string
	id           string
	err          error
	called       bool
}

func (f *fakeResolver) Resolve(ctx context.Context, apiKey, sessionID string) (string, error) {
	f.called = true
	f.gotAPIKey = apiKey
	f.gotSessionID = sessionID
	return f.id, f.err
}

// reqWithHeader builds a minimal ServerRequest carrying the given headers.
func reqWithHeader(h http.Header) mcp.Request {
	return &mcp.ServerRequest[*mcp.CallToolParams]{
		Params: &mcp.CallToolParams{Name: "noop"},
		Extra:  &mcp.RequestExtra{Header: h},
	}
}

func runMiddleware(t *testing.T, resolver TenantResolver, req mcp.Request) (string, error) {
	t.Helper()
	var seen string
	next := func(ctx context.Context, method string, r mcp.Request) (mcp.Result, error) {
		seen = tenant.From(ctx)
		return &mcp.CallToolResult{}, nil
	}
	mw := TenantMiddleware(resolver)
	_, err := mw(next)(context.Background(), "tools/call", req)
	return seen, err
}

func TestTenantMiddlewareResolvesFromHeaders(t *testing.T) {
	r := &fakeResolver{id: "t_resolved"}
	h := http.Header{}
	h.Set("Authorization", "Bearer secret-key")
	h.Set(SessionHeader, "sess-123")

	seen, err := runMiddleware(t, r, reqWithHeader(h))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.called {
		t.Fatal("expected resolver to be called")
	}
	if r.gotAPIKey != "secret-key" {
		t.Errorf("expected api key %q, got %q", "secret-key", r.gotAPIKey)
	}
	if r.gotSessionID != "sess-123" {
		t.Errorf("expected session id %q, got %q", "sess-123", r.gotSessionID)
	}
	if seen != "t_resolved" {
		t.Errorf("expected ctx tenant %q, got %q", "t_resolved", seen)
	}
}

func TestTenantMiddlewareFallsBackToXAPIKey(t *testing.T) {
	r := &fakeResolver{id: "t_x"}
	h := http.Header{}
	h.Set("X-API-Key", "xkey")
	h.Set(SessionHeader, "sess-x")

	if _, err := runMiddleware(t, r, reqWithHeader(h)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.gotAPIKey != "xkey" {
		t.Errorf("expected api key %q, got %q", "xkey", r.gotAPIKey)
	}
}

func TestTenantMiddlewareNoSessionIDUsesDefault(t *testing.T) {
	r := &fakeResolver{id: "t_never"}
	h := http.Header{}
	h.Set("Authorization", "Bearer secret-key")

	seen, err := runMiddleware(t, r, reqWithHeader(h))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.called {
		t.Error("resolver should not be called without a session id")
	}
	if seen != tenant.DefaultID {
		t.Errorf("expected default tenant, got %q", seen)
	}
}

func TestTenantMiddlewareNoHeaderPassesThrough(t *testing.T) {
	r := &fakeResolver{id: "t_never"}
	req := &mcp.ServerRequest[*mcp.CallToolParams]{Params: &mcp.CallToolParams{Name: "noop"}}

	seen, err := runMiddleware(t, r, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.called {
		t.Error("resolver should not be called without headers")
	}
	if seen != tenant.DefaultID {
		t.Errorf("expected default tenant, got %q", seen)
	}
}
