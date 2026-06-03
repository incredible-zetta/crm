# Zetta CRM — Hexagonal Refactor + Full CRUD + Compliance Plan

Date: 2026-06-03
Status: APPROVED (architecture B, soft-delete+purge, unsubscribe column)
Module: `github.com/cipta/crm-for-aiagents`
Branch: `master`

## Goal

Turn the working-but-coupled Zetta CRM MCP server into a **clean hexagonal (ports & adapters)
architecture** that an engineer (not just an AI) can read, extend, and test — while filling every
CRUD gap, adding marketing-grade statistics, and enforcing email compliance (unsubscribe).

This is a refactor + feature expansion. Behavior already verified working end-to-end (real SMTP,
live DB) must keep working. Every step is TDD with reviews.

## Why (current pain)

- Business logic (`RunCampaign`, CSV import, export orchestration, send rules) lives **inside MCP
  tool handlers** in `internal/mcptools/`. Logic is welded to the MCP transport, so it can't be
  reused by HTTP, a CLI, or tests without spinning up MCP.
- `db.Repo` is one fat struct with all entities' queries. No interface boundary → handlers depend
  on a concrete DB type.
- No domain package: the stage enum, validation, and entities are scattered in `internal/db`.
- CRUD holes: no delete anywhere, no `campaign_list`/`task_list`/`task_cancel`, no single-get.
- No unsubscribe/opt-out → not compliant for marketing email.
- Stats are all-time totals only; opens not deduped (Gmail prefetch inflates).

## Target architecture (hexagonal / ports & adapters)

```
cmd/server/                 composition root (wires adapters → services → transports)

internal/
  domain/                   ENTITIES + value objects + enums + domain errors. ZERO external deps.
    contact.go              Contact, Stage enum + transitions, ErrNotFound, ErrInvalidStage
    campaign.go             Campaign, CampaignStatus, Provider
    template.go             Template
    task.go                 ScheduledTask, TaskKind, TaskStatus
    event.go                EmailEvent, EventType
    tracking.go             TrackingLink
    export.go               Export
    errors.go               shared sentinel errors (ErrNotFound, ErrConflict, ErrValidation)

  port/                     INTERFACES (the hexagon edges). depends only on domain.
    repository.go           ContactRepo, CampaignRepo, TemplateRepo, TaskRepo, EventRepo,
                            TrackingRepo, ExportRepo  (segregated, one per aggregate)
    sender.go               EmailSender (Send)
    clock.go                Clock (Now) — injectable time
    idgen.go                IDGenerator (export ids, link codes)

  service/                  USE CASES / business logic. depends on domain + port only.
    contact_service.go      Create, Get, Update, List, Import, Export, Delete(soft), Purge, Unsubscribe
    campaign_service.go     Create, Get, List, Update, Delete, Send (the old RunCampaign), Stats
    template_service.go     Create, Get, List, Update, Delete, Render
    email_service.go        Send single (resolve recipient, unsubscribe guard, pipeline)
    task_service.go         Schedule, List, Cancel, Execute (used by scheduler worker)
    analytics_service.go    Overview, CampaignStats (dedup opens, bounce/unsub counts)
    tracking_service.go     CreateLink, ResolveClick, ResolveOpen
    service.go              Services aggregate struct + constructor (wires ports in)

  adapter/                  IMPLEMENTATIONS of ports (the "driven" side)
    mysql/                  implements port.*Repo against MySQL (moved from internal/db)
      db.go                 Open + Migrate + pool
      contact_repo.go
      campaign_repo.go
      template_repo.go
      task_repo.go
      event_repo.go
      tracking_repo.go
      export_repo.go
      mapping.go            row<->domain mapping helpers
    email/                  implements port.EmailSender (moved from internal/email)
      smtp.go  mailgun.go  sender.go  pipeline.go
    system/                 realClock, cryptoIDGen

  transport/                the "driving" side (inbound adapters)
    mcp/                    thin MCP tools: parse args → call service → format Out. (was mcptools)
      registry.go contacts.go campaigns.go templates.go email.go tasks.go
      tracking.go analytics.go ops.go respond.go auth.go server.go
    http/                   public routes: /t /o /export /healthz (was httpx)
      handlers.go

  scheduler/                in-process worker (already clean) → now calls task_service.Execute

migrations/                 0001_init (edited: add soft-delete + unsubscribe + indexes)
```

Dependency rule (inward only): `transport → service → port ← adapter`, and everyone may import
`domain`. `domain` imports nothing internal. This is the key reviewable invariant.

## Decisions (locked)

1. **Full hexagonal (Option B)** — real directory move. Worth the churn for long-term clarity.
2. **Soft delete + purge**: add `deleted_at TIMESTAMP NULL` to contacts, campaigns, templates.
   - `*_delete` sets `deleted_at` (recoverable, hidden from lists/sends/stats).
   - `contact_delete` accepts `purge: true` → hard `DELETE` row (GDPR erase).
3. **Unsubscribe**: add `unsubscribed_at TIMESTAMP NULL` to contacts.
   - `contact_unsubscribe` tool sets it; `email_send` + `campaign_send` **skip** unsubscribed.
   - Public one-click unsubscribe route `GET /u/{code}` (token = signed/contact-scoped code) so
     email recipients can opt out — appended automatically to campaign emails.
4. **Open dedup**: `analytics`/`campaign_stats` count DISTINCT contact opens (not raw pixel hits).
   Keep raw events; dedup at query time.
5. **Migrations**: edit `0001_init` (pre-prod, drop+remigrate acceptable) — single clean schema.
6. **Naming**: packages renamed (`mcptools`→`transport/mcp`, `httpx`→`transport/http`,
   `db`→`adapter/mysql`, `email`→`adapter/email`). Import paths update across the tree.

## Schema changes (edit migrations/0001_init.up.sql)

```sql
-- contacts: + soft delete + unsubscribe + unsub code
ALTER intent (applied inline in 0001):
  deleted_at TIMESTAMP NULL,
  unsubscribed_at TIMESTAMP NULL,
  unsub_code CHAR(16) NULL UNIQUE,        -- public opt-out token
  INDEX idx_contacts_deleted (deleted_at)

-- campaigns: + soft delete
  deleted_at TIMESTAMP NULL

-- email_templates: + soft delete + updated_at
  deleted_at TIMESTAMP NULL,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP

-- email_events: + 'unsubscribe' to type enum
  type ENUM('sent','delivered','open','click','bounce','failed','unsubscribe')
```

All `List`/`Get`/send/stats queries gain `WHERE deleted_at IS NULL` (except an explicit
include-deleted path if ever needed — not now).

## Tool surface (final)

Existing 16 stay (semantics preserved). New tools:

| Entity | New tools |
|--------|-----------|
| Contacts | `contact_get`, `contact_delete` (soft, `purge` flag), `contact_unsubscribe` |
| Campaigns | `campaign_list`, `campaign_get`, `campaign_update`, `campaign_delete` |
| Templates | `template_get`, `template_update`, `template_delete` |
| Tasks | `task_list`, `task_cancel` |

→ **~27 tools total.** All thin: parse → service → format. Token rules preserved
(pagination, projection, terse errors, export-as-URL).

## Stats upgrade (analytics_service)

- Overview adds: `bounced`, `failed`, `unsubscribed`, `unique_opens`.
- `campaign_stats`: opens/clicks deduped by DISTINCT contact_id; add `unsubscribed`.
- New tool `contact_engagement` (optional, stretch): per-contact sent/open/click history for
  follow-up targeting. (Include if time; not blocking.)

## Compliance flow

1. Campaign/email send: skip contacts with `unsubscribed_at IS NOT NULL` (counted as skipped).
2. Outbound HTML gets an unsubscribe footer link `{BASE_URL}/u/{unsub_code}` injected by the
   send pipeline (configurable, on by default for campaigns).
3. `GET /u/{code}` → sets `unsubscribed_at`, logs `unsubscribe` event, returns a plain confirmation page.

---

## Task breakdown (TDD, one reviewable slice each)

Each task: failing test → implement → pass → `gofmt`+`vet` → conventional commit. Reviews
(spec + code-quality subagents) after each non-trivial task, fixes folded back.

> The early tasks are a **mechanical move** (create new tree, move code, fix imports) done in a way
> that keeps the build green at each step. We do NOT rewrite logic and move it at the same time.

### Phase 1 — Domain & ports (foundation)
- **R1. domain package**: extract entities/enums/errors into `internal/domain/`. Pure structs +
  validation funcs (`Stage.Valid()`, etc.). Unit tests, no DB.
- **R2. port package**: define segregated repo interfaces + `EmailSender`, `Clock`, `IDGenerator`
  in `internal/port/`, typed against `domain`. Compile-only (interfaces).

### Phase 2 — Adapters (move existing impls behind ports)
- **R3. adapter/mysql**: move `internal/db/*` → `internal/adapter/mysql/`, make each repo implement
  its `port` interface, return `domain` types. Keep existing integration tests (repoint imports).
  Add `deleted_at`/`unsubscribe` query filters here.
- **R4. adapter/email + system**: move `internal/email/*` → `internal/adapter/email/`; add
  `system.RealClock`, `system.CryptoIDGen` implementing ports.

### Phase 3 — Services (lift logic out of handlers)
- **R5. contact_service**: Create/Get/Update/List/Import/Export/Delete/Purge/Unsubscribe.
  Move CSV import + export-file orchestration here (out of mcptools). TDD with fake ports.
- **R6. template_service**: Create/Get/List/Update/Delete/Render.
- **R7. campaign_service**: Create/Get/List/Update/Delete/Send (old RunCampaign) /Stats.
  Unsubscribe-skip enforced here.
- **R8. email_service**: single send (recipient resolution, unsubscribe guard, pipeline call).
- **R9. task_service**: Schedule/List/Cancel/Execute. Scheduler worker now calls `Execute`.
- **R10. analytics_service + tracking_service**: dedup opens, bounce/unsub counts, link create/resolve.
- **R11. service aggregate**: `service.Services` struct + constructor wiring all ports.

### Phase 4 — Transports (make handlers thin)
- **R12. transport/mcp**: move `internal/mcptools/*` → `internal/transport/mcp/`; rewrite each
  handler to call a service method. Register existing 16 tools. Tests call services via fakes or
  the real wiring.
- **R13. new CRUD tools**: add the ~11 new tools (contact_get/delete/unsubscribe, campaign_list/
  get/update/delete, template_get/update/delete, task_list/cancel) as thin handlers + service calls.
- **R14. transport/http**: move `internal/httpx/*` → `internal/transport/http/`; add `/u/{code}`
  unsubscribe route.

### Phase 5 — Schema, wiring, compliance, docs
- **R15. migration update**: edit `0001_init` (soft delete, unsubscribe, event enum); drop+remigrate
  live test DB; verify all integration tests pass.
- **R16. unsubscribe footer**: pipeline injects `/u/{code}` link into campaign HTML; send paths
  skip unsubscribed; events logged.
- **R17. main wiring**: rebuild `cmd/server` composition root against new packages; graceful
  shutdown intact; live smoke test (boot, healthz, tools/list shows ~27, send to Indra still works).
- **R18. README + ARCHITECTURE.md**: document the hexagon, dependency rule, how to add a tool/repo,
  the unsubscribe/soft-delete model. Update tool table.

### Phase 6 — (deferred, separate effort)
- MCP Resources (operator guide, schema docs) + skills book — AFTER the surface is complete.

## Verification gates

- After every phase: `go build ./...`, full `go test ./...` (with `DB_DSN`), `go vet`, `gofmt -l`.
- Dependency-rule check: `domain` imports nothing internal; `service` imports only domain+port;
  grep-assert no transport import inside service, no mysql import inside service.
- Final live e2e: recreate Indra, send real email, unsubscribe via `/u/{code}`, confirm send-skip,
  edge cases (delete→purge, soft-deleted hidden from list).

## Risks & mitigations

- **Big import churn** (renames touch every file): do moves package-by-package keeping build green;
  one commit per moved package so any break is bisectable.
- **Integration tests tied to `internal/db`**: repoint imports during R3; keep assertions identical.
- **Live DB drop+remigrate** (R15): test DB only, creds known; back up Indra row mentally (id 386
  will be recreated in final e2e). Pre-prod, acceptable.
- **Behavior regression**: the 16 existing tools have tests; they must stay green through transport
  rewrite. Reviews catch semantic drift.

## Out of scope (this plan)

- Multi-tenant / auth beyond the single API key.
- Mailgun inbound webhooks for real delivered/bounce events (events are app-generated for now).
- UI. This is an MCP + HTTP backend.
