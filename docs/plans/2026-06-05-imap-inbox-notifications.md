# IMAP Inbox + Admin Notifications Implementation Plan

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** Add a maintainable IMAP inbox integration so Zetta CRM can ingest direct lead replies, notify admins for known-contact replies, and expose inbox read/list/reply tools to AI agents.

**Architecture:** Keep the existing hexagonal rule: `transport -> service -> port <- adapter`, with `domain` shared and importing only stdlib. IMAP and notification provider code live in adapters; business rules live in `internal/service`; MCP tools remain thin transport wrappers.

**Tech Stack:** Go 1.25, MySQL 8, `github.com/modelcontextprotocol/go-sdk`, existing SMTP/Mailgun email adapter, new IMAP adapter library selected during implementation, `golang-migrate` embedded migrations.

---

## Design Summary

### MVP behavior

- One IMAP account configured by environment variables.
- Background poller plus manual MCP sync.
- Store inbound replies in MySQL idempotently.
- Match inbound sender to active CRM contact by normalized `from_email`.
- Notify admin only when message is new and from a known contact.
- Agents can sync, list, read, mark read, reply, and soft-delete local inbox messages.
- Unknown sender messages are stored but do not trigger admin notification.
- `inbox_delete` only soft-deletes local copy; it does not delete remote IMAP mail.

### Environment variables

```env
IMAP_HOST=imap.larksuite.com
IMAP_PORT=993
IMAP_USER=no-reply@zettacrm.com
IMAP_PASS=...
IMAP_MAILBOX=INBOX
IMAP_POLL_INTERVAL_SEC=60
IMAP_SINCE_DAYS=14
ADMIN_NOTIFY_EMAIL=admin@example.com
```

IMAP integration is disabled when required IMAP env is incomplete. Server must still boot.

### Data model

`inbox_cursors`:
- `id`
- `mailbox`
- `last_uid`
- `last_message_date`
- `updated_at`

`inbound_messages`:
- `id`
- `mailbox`
- `uid`
- `message_id`
- `in_reply_to`
- `references_header`
- `from_email`
- `from_name`
- `to_email`
- `subject`
- `received_at`
- `text_body`
- `html_body`
- `snippet`
- nullable `contact_id`
- nullable `campaign_id`
- nullable `read_at`
- nullable `replied_at`
- nullable `deleted_at`
- nullable `notified_at`
- `raw_headers_json`
- `created_at`

Indexes:
- unique `(mailbox, uid)`
- unique `message_id` (allow nullable/empty safely; if MySQL empty strings cause collision, store NULL for missing)
- `from_email`
- `contact_id, received_at`
- `read_at`
- `deleted_at`
- `notified_at`

Matching:
- primary: normalize `from_email` lower-case
- lookup active contact by email
- if found: set `contact_id`
- campaign matching later via `in_reply_to` / `references_header`, maybe outbound `email_events.message_id` in future
- MVP can leave `campaign_id` null unless obvious

---

## MCP Tools

### `inbox_sync`

Input:

```json
{ "limit": 50 }
```

Output:

```json
{ "fetched": 12, "new": 3, "known_contacts": 2, "notified": 2 }
```

### `inbox_list`

Input:

```json
{
  "unread": true,
  "known_only": true,
  "contact_id": 123,
  "limit": 20,
  "cursor": 0
}
```

Output uses snippets only:

```json
{
  "items": [
    {
      "id": 55,
      "from": "indra@example.com",
      "from_name": "Indra",
      "subject": "Re: Promo Website",
      "snippet": "Saya tertarik, bisa diskusi...",
      "received_at": "2026-06-05T10:00:00Z",
      "contact_id": 123,
      "read": false,
      "replied": false
    }
  ],
  "next_cursor": 55
}
```

### `inbox_get`

Input:

```json
{ "id": 55 }
```

Returns full text/html and headers.

### `inbox_mark_read`

Input:

```json
{ "id": 55, "read": true }
```

### `inbox_reply`

Input:

```json
{
  "id": 55,
  "body_text": "Halo Indra, siap. Bisa diskusi jam berapa?",
  "body_html": "<p>Halo Indra, siap. Bisa diskusi jam berapa?</p>"
}
```

Behavior:
- loads inbound message
- sends to original `from_email`
- sends from existing `SMTP_FROM` / configured sender identity
- subject keeps existing `Re:` or prefixes `Re: `
- threading headers can be added later if sender adapter supports it
- sets `replied_at` only after successful send

### `inbox_delete`

Input:

```json
{ "id": 55 }
```

Soft-deletes local message only.

---

## Implementation Tasks

### Task 1: Config for IMAP + admin notifications

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `.env.example`
- Modify: `README.md`

**Steps:**
1. Add config fields: `IMAPHost`, `IMAPPort`, `IMAPUser`, `IMAPPass`, `IMAPMailbox`, `IMAPPollIntervalSec`, `IMAPSinceDays`, `AdminNotifyEmail`.
2. Add `InboxEnabled() bool` helper returning true only when host/user/pass/mailbox/admin email are present.
3. Write failing config tests for defaults and enable/disable behavior.
4. Implement env parsing with defaults: port `993`, mailbox `INBOX`, poll interval `60`, since days `14`.
5. Update `.env.example` and README config table.
6. Run `go test ./internal/config`.
7. Commit: `feat(config): add imap inbox notification settings`.

### Task 2: Domain types + ports

**Files:**
- Create: `internal/domain/inbox.go`
- Modify: `internal/port/repository.go`
- Create/modify tests where relevant.

**Steps:**
1. Define `domain.InboundMessage`, `domain.InboxCursor`, `domain.InboxFilter`, `domain.InboxReply`.
2. Add `port.InboxRepo` with cursor/message/list/get/read/replied/delete/notified methods.
3. Add `port.InboxFetcher` interface for IMAP adapter.
4. Add `port.AdminNotifier` interface for notification adapter.
5. Add compile-time interface-friendly method signatures using `context.Context`.
6. Run `go test ./internal/domain ./internal/port`.
7. Commit: `feat(domain): add inbox message model and ports`.

### Task 3: MySQL migration + repository

**Files:**
- Modify: `migrations/0001_init.up.sql`
- Modify: `migrations/0001_init.down.sql`
- Modify: `internal/adapter/mysql/store.go` if store aggregate exists
- Create: `internal/adapter/mysql/inbox_repo.go`
- Modify: `internal/adapter/mysql/mysql_test.go`

**Steps:**
1. Add `inbox_cursors` and `inbound_messages` tables to migration.
2. Implement MySQL repo methods idempotently.
3. Insert ignores duplicates by `(mailbox, uid)` and/or `message_id`.
4. Update cursor only after successful batch handling.
5. Add integration tests for insert duplicate, list pagination, mark read, mark replied, soft delete, notification retry query.
6. Run `go test ./internal/adapter/mysql`.
7. Commit: `feat(mysql): persist inbound inbox messages`.

### Task 4: MIME parser package

**Files:**
- Create: `internal/adapter/imap/parser.go`
- Create: `internal/adapter/imap/parser_test.go`
- Create: `internal/adapter/imap/testdata/*.eml`

**Steps:**
1. Write parser tests from raw MIME fixtures: plain text, HTML, multipart alternative, missing date/message-id.
2. Implement parser returning `domain.InboundMessage` fields.
3. Create snippet from text, fallback stripped HTML if no text.
4. Normalize sender email lower-case.
5. Store useful raw headers JSON.
6. Run `go test ./internal/adapter/imap`.
7. Commit: `feat(imap): parse inbound email messages`.

### Task 5: IMAP fetcher adapter

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/adapter/imap/fetcher.go`
- Create: `internal/adapter/imap/fetcher_test.go` if library supports fakes; otherwise keep parser tests and use service fake fetcher.

**Steps:**
1. Select small maintained IMAP library; keep adapter boundary narrow.
2. Implement TLS connect, login, select mailbox.
3. Fetch `UID > last_uid`, with first-run fallback since `now - IMAP_SINCE_DAYS` and limit cap.
4. Parse messages with parser.
5. Return fetched messages and max UID.
6. Do not require live IMAP in CI.
7. Run `go test ./internal/adapter/imap`.
8. Commit: `feat(imap): fetch new mailbox messages`.

### Task 6: Inbox service

**Files:**
- Create: `internal/service/inbox_service.go`
- Create: `internal/service/inbox_service_test.go`
- Modify: `internal/service/services.go`
- Modify: `internal/service/services_test.go`

**Steps:**
1. Write fake repo/fetcher/notifier tests for sync flow.
2. Test unknown sender stored but not notified.
3. Test known contact stored and notified once.
4. Test notification failure does not lose message and remains retryable.
5. Test list/get/read/delete/reply behavior.
6. Implement service methods.
7. `Reply()` sends via existing `EmailSender`, then sets `replied_at`.
8. Run `go test ./internal/service`.
9. Commit: `feat(service): add inbox sync and reply use cases`.

### Task 7: Admin notifier adapter

**Files:**
- Create: `internal/adapter/email/admin_notifier.go`
- Create: `internal/adapter/email/admin_notifier_test.go`

**Steps:**
1. Implement notifier using existing `port.EmailSender`.
2. Email subject: `Zetta CRM: New reply from {contact/email}`.
3. Body includes contact, subject, snippet, received date, suggested MCP tools.
4. Never include secrets.
5. Run `go test ./internal/adapter/email`.
6. Commit: `feat(email): add admin inbox notifications`.

### Task 8: Inbox poller

**Files:**
- Create: `internal/inboxpoller/worker.go`
- Create: `internal/inboxpoller/worker_test.go`
- Modify: `cmd/server/main.go`

**Steps:**
1. Implement worker loop like scheduler: start, run once, ticker, context cancellation.
2. Worker calls `InboxService.Sync(ctx, limit=100)`.
3. Log errors, never panic.
4. Wire in `cmd/server/main.go` only when `cfg.InboxEnabled()`.
5. Debug logs include sync counts.
6. Server still boots when IMAP disabled.
7. Run `go test ./internal/inboxpoller ./cmd/server`.
8. Commit: `feat(inbox): add background imap poller`.

### Task 9: MCP inbox tools

**Files:**
- Create: `internal/transport/mcp/inbox.go`
- Modify: `internal/transport/mcp/registry.go`
- Modify: `internal/transport/mcp/mcp_test.go` or create focused test file.

**Steps:**
1. Add input/output structs for `inbox_sync`, `inbox_list`, `inbox_get`, `inbox_mark_read`, `inbox_reply`, `inbox_delete`.
2. Register tools.
3. Keep outputs compact; list returns snippets only.
4. Use terse errors `{error,msg}`.
5. Add MCP tests for sync/list/get/reply/error cases.
6. Run `go test ./internal/transport/mcp`.
7. Commit: `feat(mcp): expose inbox tools to agents`.

### Task 10: Composition root + docs

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `ARCHITECTURE.md`
- Modify: `README.md`
- Modify: `docs/wiki/Installation.md`
- Modify relevant EasyPanel docs in `docs/wiki/`.

**Steps:**
1. Wire MySQL inbox repo, IMAP fetcher, admin notifier, inbox service, poller.
2. Ensure disabled IMAP path logs warning only in debug or concise startup log.
3. Update architecture docs with inbox flow.
4. Update README tools count and MCP tool list.
5. Add EasyPanel env instructions.
6. Run `make check` and `staticcheck ./...`.
7. Commit: `docs: document imap inbox and admin notifications`.

### Task 11: End-to-end local smoke script

**Files:**
- Modify: `scripts/test-mcp.sh` or create `scripts/test-inbox.sh`

**Steps:**
1. Add optional smoke flow for `inbox_sync`, `inbox_list`, `inbox_get`, `inbox_reply`.
2. Skip IMAP tests if IMAP env missing.
3. Keep secrets out of logs.
4. Run script against local/live only when env present.
5. Commit: `test: add optional inbox smoke script`.

### Task 12: Final verification + release

**Steps:**
1. Run `gofmt` on touched Go files.
2. Run `make check`.
3. Run `$(go env GOPATH)/bin/staticcheck ./...`.
4. Push `master`.
5. Create beta release tag, likely `v0.1.0-beta` or next patch if user wants incremental beta.
6. Confirm Release Images workflow success.
7. Update landing site release tag if version changes.

---

## Risk Notes

- IMAP library choice should stay behind `port.InboxFetcher`; no library types leak into service/domain.
- Do not log `IMAP_PASS`, `SMTP_PASS`, API keys, or full DSN.
- Message bodies can be large; list must return snippets only.
- Notification failure must never block storage.
- Unknown sender replies can be useful later; store them but do not notify admin in MVP.
- Campaign matching is intentionally deferred; maybe outbound `email_events.message_id` later.
