// Package tenant carries the active tenant identity through context.Context.
//
// Multi-tenancy is optional and controlled by the MULTI_TENANCY env flag
// (see internal/config). When disabled, every request and background job runs
// as the implicit single tenant DefaultID, preserving the original
// single-tenant behavior. When enabled, an MCP middleware resolves the caller
// from the (Authorization, session id) pair, auto-provisions a tenant row on
// first contact, and stamps the resolved tenant id into the request context.
//
// The mysql adapter reads the tenant id from context on every query so that
// data is transparently scoped. Code paths that must operate across all
// tenants (e.g. the scheduler claiming due tasks) use the raw repo without a
// tenant in context and are responsible for re-injecting the per-row tenant
// before doing tenant-scoped work.
package tenant

import "context"

// DefaultID is the implicit tenant used when multi-tenancy is disabled, and
// the fallback for background contexts that have not had a tenant injected.
const DefaultID = "default"

type ctxKey struct{}

// With returns a copy of ctx carrying the given tenant id. An empty id is
// normalized to DefaultID so downstream queries always have a concrete scope.
func With(ctx context.Context, id string) context.Context {
	if id == "" {
		id = DefaultID
	}
	return context.WithValue(ctx, ctxKey{}, id)
}

// From returns the tenant id carried by ctx, or DefaultID when none is set.
func From(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKey{}).(string); ok && v != "" {
		return v
	}
	return DefaultID
}
