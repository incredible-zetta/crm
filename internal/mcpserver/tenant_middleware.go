package mcpserver

import (
	"context"
	"strings"

	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/tenant"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SessionHeader is the client-controlled, static-per-tenant header used to
// distinguish tenants alongside the Authorization key. Unlike Mcp-Session-Id
// (which the SDK auto-generates after initialize and the client cannot set
// ahead of connect), X-Session-Id is fully client-controlled and stable, so a
// tenant always presents the same value.
const SessionHeader = "X-Session-Id"

// TenantResolver resolves (and auto-provisions) a tenant id from caller
// credentials. The mysql adapter implements this via port.TenantRepo.
type TenantResolver interface {
	Resolve(ctx context.Context, apiKey, sessionID string) (string, error)
}

var _ TenantResolver = (port.TenantRepo)(nil)

// TenantMiddleware returns an MCP receiving middleware that resolves the
// active tenant from each request's headers and injects it into the context
// that tool handlers receive. It is only wired when multi-tenancy is enabled;
// in single-tenant mode tool handlers run under tenant.DefaultID with no
// resolver consulted.
//
// Resolution uses the (Authorization/X-API-Key token, X-Session-Id) pair. A
// missing session id falls back to tenant.DefaultID so a misconfigured client
// degrades to the shared default tenant rather than leaking across tenants.
func TenantMiddleware(resolver TenantResolver) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			extra := req.GetExtra()
			if extra == nil || extra.Header == nil {
				return next(ctx, method, req)
			}

			sessionID := strings.TrimSpace(extra.Header.Get(SessionHeader))
			if sessionID == "" {
				// No session id: stay on the default tenant.
				return next(tenant.With(ctx, tenant.DefaultID), method, req)
			}

			apiKey := bearerOrAPIKey(extra.Header.Get("Authorization"), extra.Header.Get("X-API-Key"))

			id, err := resolver.Resolve(ctx, apiKey, sessionID)
			if err != nil {
				return nil, err
			}
			return next(tenant.With(ctx, id), method, req)
		}
	}
}

// bearerOrAPIKey extracts the API key from the Authorization bearer header,
// falling back to the X-API-Key header. Mirrors AuthHandler's parsing.
func bearerOrAPIKey(authHeader, xAPIKey string) string {
	if len(authHeader) >= 7 && strings.EqualFold(authHeader[:7], "bearer ") {
		return authHeader[7:]
	}
	return xAPIKey
}
