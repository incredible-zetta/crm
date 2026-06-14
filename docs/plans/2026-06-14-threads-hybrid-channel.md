# Threads Hybrid Channel Implementation Plan

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** Add Threads Graph API MCP tools with live calls plus MySQL cache/audit storage.

**Architecture:** Follow existing hexagonal shape: domain models, port interfaces, live Graph API adapter, MySQL repo, service orchestration, thin MCP transport. Live Graph API stays source of truth; cache/audit writes are best-effort where possible. Cached rows store typed columns plus raw JSON for API freshness.

**Tech Stack:** Go 1.23, net/http, MySQL migrations, MCP Go SDK, existing service/port/domain layers.

---

### Task 1: Config + ports + domain

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/domain/threads.go`
- Create: `internal/port/threads.go`

**Steps:**
1. Add `THREADS_ACCESS_TOKEN`, `THREADS_USER_ID`, `THREADS_API_VERSION` config fields and `ThreadsEnabled()`.
2. Add domain structs for profile, post, reply, mention, insight, audit event, filters.
3. Add gateway and repo ports.
4. Run `go test ./internal/config ./internal/domain ./internal/port`.

### Task 2: Live Threads adapter

**Files:**
- Create: `internal/adapter/threads/client.go`
- Create: `internal/adapter/threads/client_test.go`

**Steps:**
1. Write httptest coverage for URL building, token attachment, error mapping, publish two-step flow.
2. Implement adapter methods: profile, list, publish, delete, insights, replies, reply, mentions, search/profile discovery.
3. Run `go test ./internal/adapter/threads`.

### Task 3: MySQL cache/audit repo

**Files:**
- Create: `migrations/0006_threads.up.sql`
- Create: `migrations/0006_threads.down.sql`
- Modify: `internal/adapter/mysql/db.go`
- Create: `internal/adapter/mysql/threads_repo.go`

**Steps:**
1. Add tables `threads_posts`, `threads_replies`, `threads_mentions`, `threads_audit_events`, all with `raw_json` where relevant.
2. Implement upsert/list/get/delete/audit methods.
3. Run `go test ./internal/adapter/mysql`.

### Task 4: Threads service

**Files:**
- Modify: `internal/service/services.go`
- Create: `internal/service/threads_service.go`
- Create: `internal/service/threads_service_test.go`

**Steps:**
1. Write fake gateway/repo tests for live calls writing cache/audit best-effort.
2. Implement service methods.
3. Wire into `service.Services`.
4. Run `go test ./internal/service`.

### Task 5: MCP tools + docs + e2e

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `internal/transport/mcp/registry.go`
- Create: `internal/transport/mcp/threads.go`
- Modify: `README.md`

**Steps:**
1. Register tools: `threads_profile`, `threads_list`, `threads_publish`, `threads_delete`, `threads_insights`, `threads_replies`, `threads_reply`, `threads_mentions`, `threads_search`, `threads_list_cached`, `threads_get_cached`, `threads_history`, `threads_delete_cached`.
2. Wire adapter/repo when `ThreadsEnabled()`.
3. Document env and tools.
4. Run `go test ./...`, `go vet ./...`, `go build ./...`.
5. E2E with `/home/nst/WebstormProjects/threads-zee/.env` token by exporting env locally; never commit token.
