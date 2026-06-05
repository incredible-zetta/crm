# Architecture

Zetta CRM is built as a **hexagonal (ports & adapters)** application. The goal is
that an engineer can open any layer and understand it in isolation, swap an
adapter without touching business logic, and test the core with no database.

## The dependency rule

Dependencies point **inward** only:

```
        ┌─────────────────────────────────────────────┐
        │                  transport                    │  inbound adapters
        │   transport/mcp (28 MCP tools)                │  (driving side)
        │   transport/http (/t /o /export /u /healthz)  │
        └───────────────────┬───────────────────────────┘
                            │ calls
        ┌───────────────────▼───────────────────────────┐
        │                   service                      │  use cases / business logic
        │   Contact, Template, Campaign, Email, Task,    │
        │   Analytics, Tracking + Services aggregate     │
        └───────────────────┬───────────────────────────┘
                            │ depends on
        ┌───────────────────▼───────────────────────────┐
        │                    port                        │  interfaces (the hexagon edges)
        │   ContactRepo, CampaignRepo, ... EmailSender,  │
        │   Clock, IDGenerator                           │
        └───────────────────┬───────────────────────────┘
                            │ implemented by
        ┌───────────────────▼───────────────────────────┐
        │                   adapter                      │  outbound adapters
        │   adapter/mysql   (repositories)               │  (driven side)
        │   adapter/email   (SMTP + Mailgun senders)     │
        │   adapter/system  (clock, crypto id generator) │
        └────────────────────────────────────────────────┘

        ┌────────────────────────────────────────────────┐
        │  domain  — entities, enums, sentinel errors      │  imported by everyone,
        │  (Contact, Campaign, Stage, ErrNotFound, ...)    │  imports nothing internal
        └────────────────────────────────────────────────┘
```

Concretely, enforced and verifiable with `go list`:

| Package | May import | Must NOT import |
|---------|-----------|-----------------|
| `internal/domain` | std lib only | anything internal |
| `internal/port` | `domain`, std lib | service, adapter, transport |
| `internal/service` | `domain`, `port`, `template` (pure util), std lib | adapter, transport, `database/sql` |
| `internal/adapter/*` | `domain`, `port`, drivers | service, transport |
| `internal/transport/*` | `service`, `domain`, `mcpserver`, std lib | adapter, `database/sql` |
| `cmd/server` | everything (composition root) | — |

The composition root (`cmd/server`) is the only place that knows about concrete
adapters. It wires them into the service layer and hands the services to the
transports.

## Packages

```
cmd/server/            composition root: build adapters → service.New(...) → transports; graceful shutdown
                       adapters.go: scheduler glue (Task<->domain) + disabledSender fallback

internal/
  domain/              entities + typed enums (Stage, Provider, CampaignStatus, TaskKind,
                       TaskStatus, EventType) + sentinel errors (ErrNotFound, ErrConflict,
                       ErrValidation). Pure; zero internal deps.

  port/                repository.go  — ContactRepo, CampaignRepo, TemplateRepo, TaskRepo,
                                        EventRepo, TrackingRepo, ExportRepo (segregated)
                       sender.go      — EmailSender + OutboundMessage
                       clock.go       — Clock
                       idgen.go       — IDGenerator

  service/             one file per use-case service; services.go assembles them.
                       All business logic lives here: validation, the send pipeline,
                       campaign expansion, CSV import/export, unsubscribe enforcement,
                       analytics aggregation. Tested with in-memory fakes — no DB.

  adapter/
    mysql/             implements every port.*Repo against MySQL. Store.Contacts() etc.
                       return the interfaces. Soft-delete + unsubscribe filtering lives here.
    email/             SMTP + Mailgun senders implementing port.EmailSender.
    system/            RealClock + CryptoIDGen.

  template/            pure text/template + HTML utilities (render, link rewrite, open
                       pixel, unsubscribe footer). No internal deps; shared by service.

  scheduler/           in-process worker (claim → execute → mark). Interface-driven;
                       the composition root adapts it to TaskService.Execute.

  mcpserver/           MCP server construction, API-key auth handler, response helpers
                       (terse {error,msg} envelopes).

  transport/
    mcp/               28 thin MCP tool handlers: parse args → call a service → format Out.
    http/              public routes: click, open pixel, export download, unsubscribe, health.

migrations/            embedded SQL (0001_init), applied on startup.
```

## Error contract

Services wrap three domain sentinels. Transports translate them:

| Service returns | MCP transport | HTTP transport |
|-----------------|---------------|----------------|
| `domain.ErrValidation` | `{error:"invalid_input", msg}` (terse) | 400 / safe page |
| `domain.ErrNotFound` | `{error:"not_found", msg}` | 404 |
| `domain.ErrConflict` | `{error:"conflict", msg}` | 409 |
| any other error (infra) | Go error (surfaced as tool error; no raw text leaked) | 500 |

Raw infrastructure errors (DB, SMTP) are never placed verbatim into a client
response. Email send failures map to a generic `{error:"send_failed"}`.

## Compliance & soft delete

- **Soft delete**: `contacts`, `campaigns`, `email_templates` carry `deleted_at`.
  Repositories filter `deleted_at IS NULL` from all reads. `contact_delete`
  supports `purge: true` for a hard GDPR delete.
- **Unsubscribe**: a contact has `unsubscribed_at` + a public `unsub_code`.
  - `email_send` / `campaign_send` skip unsubscribed contacts (the send pipeline
    refuses; campaign send counts them as `skipped`).
  - Campaign emails get a per-contact unsubscribe footer linking to
    `{BASE_URL}/u/{code}`.
  - `GET /u/{code}` sets `unsubscribed_at`, logs an `unsubscribe` event, and
    shows a confirmation page without revealing whether the code existed.

## Adding things

**A new tool**: add a thin handler in `transport/mcp`, register it in
`registry.go`, and call an existing (or new) service method. No DB code in the
handler.

**A new repository method**: add it to the relevant `port` interface, implement
it in `adapter/mysql`, then use it from a service. The compile-time
`var _ port.XxxRepo = (*xxxRepo)(nil)` asserts keep the adapter honest.

**A different database / mail provider**: implement the `port` interfaces in a
new `adapter/` package and wire it in `cmd/server`. Nothing in `service`,
`domain`, or `transport` changes.

## Testing strategy

- `domain`, `service`, `transport`, `template`, `scheduler` — pure unit tests
  with fakes; run without a database.
- `adapter/mysql` — integration tests against a live MySQL; they `t.Skip` when
  `DB_DSN` is unset.

```bash
go test ./...                 # unit layers always run; mysql skips without DB_DSN
DB_DSN='...' go test ./...     # full suite incl. mysql integration
```

## Inbox / IMAP flow

Inbound email follows same hexagonal boundary:

```text
IMAP server -> adapter/imap -> port.InboxFetcher -> service.InboxService -> port.InboxRepo -> adapter/mysql
                                                        |-> port.AdminNotifier -> adapter/email
MCP inbox tools -> transport/mcp -> service.InboxService
```

Rules:
- IMAP is optional. Missing IMAP/admin env disables inbox; server still boots.
- Poller is in-process and calls `InboxService.Sync`; errors are logged, never panic.
- Sender matching is by normalized `from_email` against active contacts.
- Admin notifications fire only for new messages from known contacts.
- `inbox_list` returns snippets; `inbox_get` returns full body.
- `inbox_delete` soft-deletes only local MySQL copy; remote IMAP mail is untouched.
