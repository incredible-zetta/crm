# CRM-for-AI-Agents MCP — Implementation Plan

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** Build a Go MCP server (Streamable HTTP) backed by MySQL that exposes 16 CRM tools for AI Agents — contacts, email, campaigns, templates, tracking, scheduler, analytics — deployable to a VPS via a single Dockerfile (EasyPanel).

**Architecture:** Single Go binary. One HTTP listener with two route groups: `/mcp` (key-gated MCP Streamable HTTP) and public tracking/export routes (`/t/{code}`, `/o/{code}.png`, `/export/{id}.csv`, `/healthz`). MySQL store with embedded migrations run on boot. In-process goroutine scheduler polls a MySQL job table (restart-safe, no Redis). Email via SMTP or Mailgun behind a `Sender` interface. Token-optimized tool responses (pagination, field projection, export-as-URL, terse errors).

**Tech Stack:** Go 1.23, `github.com/modelcontextprotocol/go-sdk` (v1.4.0+), `github.com/go-sql-driver/mysql`, `github.com/golang-migrate/migrate/v4` (embedded source), `html/template`/`text/template`, distroless Docker image.

**Design ref:** `docs/plans/2026-06-03-crm-mcp-design.md`

---

## Conventions

- TDD: write failing test → run (fail) → minimal impl → run (pass) → commit.
- Tests use a real MySQL test DB (`crmagents`) via `DB_DSN` env, transactions rolled back where possible, or a `crmagents_test` schema. Skip integration tests if `DB_DSN` unset (`t.Skip`).
- Commit after every passing task with conventional messages.
- Run `go vet ./...` and `gofmt -l .` before each commit.
- All MCP tool handlers return terse errors `{error,msg}` shape via a helper, never raw stack traces.

---

## Task 0: Repo bootstrap

**Files:**
- Create: `go.mod`, `.env.example`, `README.md`

**Step 1: Init module**

```bash
cd /home/nst/WebstormProjects/crm-for-aiagents
go mod init github.com/cipta/crm-for-aiagents
```

**Step 2: Add deps**

```bash
go get github.com/modelcontextprotocol/go-sdk/mcp@latest
go get github.com/go-sql-driver/mysql@latest
go get github.com/golang-migrate/migrate/v4@latest
```

**Step 3: Write `.env.example`**

```
MCP_API_KEY=changeme-long-random
BASE_URL=https://crm.example.com
DB_DSN=crmagents_user_bf2722:PASSWORD@tcp(localhost:3306)/crmagents?parseTime=true&multiStatements=true
SMTP_HOST=
SMTP_PORT=587
SMTP_USER=
SMTP_PASS=
SMTP_FROM=
MAILGUN_DOMAIN=
MAILGUN_API_KEY=
PORT=8080
SCHEDULER_INTERVAL_SEC=15
```

**Step 4: Commit**

```bash
go mod tidy
git add go.mod go.sum .env.example .gitignore README.md
git commit -m "chore: bootstrap go module and env example"
```

---

## Task 1: Config loader

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Step 1: Failing test**

```go
package config

import (
	"os"
	"testing"
)

func TestLoadRequiresAPIKey(t *testing.T) {
	os.Clearenv()
	if _, err := Load(); err == nil {
		t.Fatal("expected error when MCP_API_KEY missing")
	}
}

func TestLoadDefaults(t *testing.T) {
	os.Clearenv()
	os.Setenv("MCP_API_KEY", "k")
	os.Setenv("DB_DSN", "dsn")
	os.Setenv("BASE_URL", "http://x")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != "8080" {
		t.Fatalf("want default port 8080, got %s", c.Port)
	}
	if c.SchedulerIntervalSec != 15 {
		t.Fatalf("want default 15, got %d", c.SchedulerIntervalSec)
	}
}
```

**Step 2: Run** `go test ./internal/config/ -v` → FAIL (no Load).

**Step 3: Implement** `Config` struct + `Load()` reading env, validating `MCP_API_KEY`, `DB_DSN`, `BASE_URL` required; defaults `PORT=8080`, `SCHEDULER_INTERVAL_SEC=15`; SMTP/Mailgun optional.

**Step 4: Run** `go test ./internal/config/ -v` → PASS.

**Step 5: Commit** `feat: add config loader`.

---

## Task 2: DB connection + embedded migrations

**Files:**
- Create: `internal/db/db.go`, `migrations/0001_init.up.sql`, `migrations/0001_init.down.sql`, `migrations/embed.go`
- Test: `internal/db/db_test.go`

**Step 1:** Write `migrations/0001_init.up.sql` with all 7 tables from design §4. `.down.sql` drops them.

**Step 2:** `migrations/embed.go`:

```go
package migrations
import "embed"
//go:embed *.sql
var FS embed.FS
```

**Step 3: Failing test** (skips if no DB_DSN):

```go
func TestOpenAndMigrate(t *testing.T) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" { t.Skip("no DB_DSN") }
	d, err := Open(dsn)
	if err != nil { t.Fatal(err) }
	if err := Migrate(d, "crmagents"); err != nil { t.Fatal(err) }
	var n int
	if err := d.QueryRow("SELECT COUNT(*) FROM contacts").Scan(&n); err != nil {
		t.Fatalf("contacts table missing: %v", err)
	}
}
```

**Step 4: Run** → FAIL.

**Step 5: Implement** `Open(dsn) (*sql.DB,error)` (ping, set pool limits) and `Migrate(db,dbname)` using `golang-migrate` with `iofs` source over `migrations.FS` + `mysql` driver.

**Step 6: Run** with `DB_DSN` set → PASS.

**Step 7: Commit** `feat: add mysql connection and embedded migrations`.

---

## Task 3: Repository layer — contacts

**Files:**
- Create: `internal/db/contacts.go`
- Test: `internal/db/contacts_test.go`

**Step 1: Failing tests** (skip w/o DB): `UpsertContact` inserts then updates by email; `ListContacts` paginates with limit+cursor and filters by stage; invalid stage rejected.

**Step 2: Run** → FAIL.

**Step 3: Implement** `Contact` struct, `UpsertContact`, `GetContact`, `UpdateContact` (validates stage enum), `ListContacts(filter, limit, cursor) (items, total, nextCursor)`, `CountByStage`.

**Step 4: Run** → PASS.

**Step 5: Commit** `feat: add contacts repository`.

---

## Task 4: Repository layer — templates, campaigns, events, tasks, tracking, exports

**Files:**
- Create: `internal/db/templates.go`, `campaigns.go`, `events.go`, `tasks.go`, `tracking.go`, `exports.go`
- Test: matching `_test.go` per file

**Step 1–4 (repeat per file, TDD):**
- templates: create, get, list.
- campaigns: create, get, updateStatus, setStats.
- events: insert (`sent/open/click/...`), aggregates `OverviewCounts`, `CampaignCounts`, `TopLinks`.
- tasks: insert, `ClaimDue(now,limit)` using `FOR UPDATE SKIP LOCKED`, markDone/markFailed.
- tracking: `CreateLink(targetURL,...) -> code`, `GetLink(code)`.
- exports: create row, get path by id.

**Step 5: Commit** each file group: `feat: add <name> repository`.

---

## Task 5: Email — Sender interface + SMTP + Mailgun

**Files:**
- Create: `internal/email/sender.go`, `smtp.go`, `mailgun.go`
- Test: `internal/email/smtp_test.go`, `internal/email/mailgun_test.go`

**Step 1: Failing tests** — SMTP builds correct RFC822 message (assert headers/body via a fake `net/smtp` send func injected); Mailgun posts multipart to `https://api.mailgun.net/v3/{domain}/messages` with basic auth (use `httptest.Server` to capture request).

**Step 2: Run** → FAIL.

**Step 3: Implement** `Sender` interface `Send(ctx, msg Message) error`; `Message{To,From,Subject,HTML,Text}`; `SMTPSender` (configurable dialer), `MailgunSender` (httpClient injectable). Factory `New(provider, cfg)`.

**Step 4: Run** → PASS.

**Step 5: Commit** `feat: add smtp and mailgun senders`.

---

## Task 6: Template render + link rewrite + open pixel

**Files:**
- Create: `internal/template/render.go`
- Test: `internal/template/render_test.go`

**Step 1: Failing tests:**
- `Render(tmpl, vars)` substitutes `{{.FirstName}}`.
- `RewriteLinks(html, baseURL, makeCode)` replaces `<a href="X">` with `baseURL/t/{code}` and records mapping.
- `InjectPixel(html, baseURL, code)` appends `<img src="baseURL/o/{code}.png" width="1" height="1">`.

**Step 2: Run** → FAIL.

**Step 3: Implement** using `html/template` for body, a link-rewrite pass (regex or `golang.org/x/net/html` tokenizer), pixel injection before `</body>`.

**Step 4: Run** → PASS.

**Step 5: Commit** `feat: add template render, link rewrite, open pixel`.

---

## Task 7: Send pipeline (ties email + template + tracking + events)

**Files:**
- Create: `internal/email/pipeline.go`
- Test: `internal/email/pipeline_test.go`

**Step 1: Failing test** — `SendToContact` with a fake Sender + in-memory repos: renders template, rewrites links (creates tracking_links rows), injects pixel, calls Sender, logs `sent` event. On Sender error logs `failed`.

**Step 2: Run** → FAIL.

**Step 3: Implement** `Pipeline` struct depending on small interfaces (TrackingRepo, EventRepo, Sender, TemplateRepo). Keep DB-agnostic for testability.

**Step 4: Run** → PASS.

**Step 5: Commit** `feat: add email send pipeline`.

---

## Task 8: Tracking + export + health HTTP handlers

**Files:**
- Create: `internal/tracking/handlers.go`, `internal/export/handlers.go`, `internal/httpx/health.go`
- Test: `internal/tracking/handlers_test.go`, `internal/export/handlers_test.go`

**Step 1: Failing tests** via `httptest`:
- `GET /t/{code}` → 302 to target_url, logs `click` event.
- `GET /o/{code}.png` → 200 image/gif 1x1 bytes, logs `open`.
- `GET /export/{id}.csv` → 200 text/csv from stored file; 404 if missing/expired.
- `GET /healthz` → 200 `ok`.

**Step 2: Run** → FAIL.

**Step 3: Implement** handlers (use `net/http` + `chi` or std mux with path params; std `http.ServeMux` Go 1.22 patterns `GET /t/{code}` work). 1x1 gif as embedded bytes.

**Step 4: Run** → PASS.

**Step 5: Commit** `feat: add tracking, export, health handlers`.

---

## Task 9: MCP server + auth + error helper

**Files:**
- Create: `internal/mcp/server.go`, `internal/mcp/auth.go`, `internal/mcp/respond.go`
- Test: `internal/mcp/auth_test.go`

**Step 1: Failing test** — `getServer` returns nil/401 path when Authorization header missing/wrong; passes with correct `MCP_API_KEY`.

**Step 2: Run** → FAIL.

**Step 3: Implement:**
- `NewMCPServer(deps)` builds `mcp.NewServer` and registers all tools (Task 10).
- `Handler(cfg, server)` wraps `mcp.NewStreamableHTTPHandler` with an `http.Handler` that checks `Authorization: Bearer` / `X-API-Key` == `cfg.MCPAPIKey` → else 401.
- `respond.go`: `Err(code,msg)` helper producing terse CallToolResult; compact JSON marshal helper.

**Step 4: Run** → PASS.

**Step 5: Commit** `feat: add mcp server, key auth, response helpers`.

---

## Task 10: Register 16 tools

**Files:**
- Create: `internal/mcp/tools/contacts.go`, `email.go`, `campaigns.go`, `templates.go`, `tracking.go`, `scheduler.go`, `ops.go`, `analytics.go`
- Test: `internal/mcp/tools/*_test.go` (call handlers directly with fake deps)

For each tool: define typed `Input`/`Output` structs with `jsonschema` tags, handler `func(ctx, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error)`, register via `mcp.AddTool(server, &mcp.Tool{Name,Description}, handler)`.

Tools (design §5): contact_create, contact_update, contact_list, contact_import, contact_export, email_send, campaign_create, campaign_send, template_create, template_list, template_render, tracking_link_create, schedule_task, health_check, analytics_overview, campaign_stats.

**Per tool TDD:** failing handler test with fake deps asserting happy path + token rules (pagination defaults, fields projection, export returns URL, render text-default). Implement. Pass. Commit per group: `feat: add <group> mcp tools`.

**Token-rule tests to include:**
- `contact_list` defaults limit 20, caps at 100, returns `{total,count,items,next_cursor}`.
- `contact_list` honors `fields` projection.
- `contact_export` returns `url` not inline rows.
- `template_render` returns text by default, html only when `html=true`.
- error helper returns `{error,msg}` on bad stage.

---

## Task 11: Scheduler worker

**Files:**
- Create: `internal/scheduler/worker.go`
- Test: `internal/scheduler/worker_test.go`

**Step 1: Failing test** — with fake task repo + pipeline: `RunOnce` claims due pending tasks, executes (email/campaign), marks done; on error marks failed and increments attempts; respects max attempts.

**Step 2: Run** → FAIL.

**Step 3: Implement** `Worker{repo,pipeline,interval}` with `RunOnce(ctx)` and `Start(ctx)` ticker loop calling `RunOnce`.

**Step 4: Run** → PASS.

**Step 5: Commit** `feat: add scheduler worker`.

---

## Task 12: main wiring

**Files:**
- Create: `cmd/server/main.go`

**Step 1:** Load config → open DB → migrate → build repos → build sender/pipeline → build MCP server+tools → build router (mount `/mcp` handler + tracking/export/health) → start scheduler goroutine → `http.ListenAndServe(:PORT)`.

**Step 2:** Manual run: `go run ./cmd/server` with `.env` (use `godotenv` or shell export). Confirm `/healthz` 200.

**Step 3: Commit** `feat: wire main server entrypoint`.

---

## Task 13: MCP smoke-test script

**Files:**
- Create: `scripts/test-mcp.sh`

**Step 1:** Bash script: curl `POST /mcp` `initialize`, then `tools/list`, then `tools/call health_check` with `Authorization: Bearer $MCP_API_KEY`. Print responses.

**Step 2:** Run against local server → see 16 tools + health JSON.

**Step 3: Commit** `chore: add mcp smoke test script`.

---

## Task 14: Dockerfile (EasyPanel)

**Files:**
- Create: `Dockerfile`, `.dockerignore`

**Step 1:** Multi-stage:

```dockerfile
FROM golang:1.23 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /crm-server ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /crm-server /crm-server
EXPOSE 8080
ENTRYPOINT ["/crm-server"]
```

`.dockerignore`: `.git`, `*.txt`, `.env`, `docs`, `*_test.go` optional.

**Step 2:** Build locally: `docker build -t crm-mcp .` → succeeds.

**Step 3:** Run: `docker run --env-file .env -p 8080:8080 crm-mcp`, hit `/healthz`.

**Step 4: Commit** `feat: add dockerfile for easypanel deploy`.

---

## Task 15: README + deploy notes

**Files:**
- Modify: `README.md`

Document: env vars, EasyPanel setup (Dockerfile app, set domain → port 8080, inject env, ensure MySQL reachable), how agents connect (`/mcp` URL + Bearer key), tracking domain note (BASE_URL must be public domain), smoke test usage.

**Commit** `docs: add readme and deploy notes`.

---

## Done criteria

- `go test ./...` green (integration tests pass with DB_DSN set).
- `go vet ./...` clean, `gofmt -l .` empty.
- `docker build` succeeds; container serves `/healthz` and `/mcp`.
- `scripts/test-mcp.sh` lists 16 tools and returns health_check JSON.
