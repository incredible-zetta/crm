<div align="center">

<img src="assets/brand/zetta-logo.png" alt="Zetta CRM" height="96" />

# Zetta CRM

**Self-hosted CRM for AI operators.**

A Go [Model Context Protocol](https://modelcontextprotocol.io) server that gives any AI agent a full CRM: contacts, email (SMTP or Mailgun), marketing campaigns, click/open tracking, inbound inbox, WhatsApp, Threads publishing/replies, scheduling, templates, and analytics. Single binary, single MySQL database, single Docker image. Built for one-port deployment on EasyPanel.

<sub>A partnership between <b>Incredible Zetta</b> and <a href="https://github.com/cds-id">Ciptadusa (CDS)</a></sub>

<br/>
<img src="assets/brand/cds-logo.png" alt="Ciptadusa" height="40" />

</div>

---

## Transport

Streamable HTTP (MCP SDK [`github.com/modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk)). The MCP endpoint is `POST /mcp`, gated by an API key. Public, unauthenticated routes serve link tracking, the open pixel, CSV export downloads, and health.

| Route | Auth | Purpose |
|-------|------|---------|
| `POST /mcp` | API key | MCP JSON-RPC (Streamable HTTP) |
| `GET /t/{code}` | public | Click tracking → 302 redirect to target |
| `GET /o/{code}.png` | public | Open-tracking 1×1 pixel |
| `GET /export/{id}.csv` | public | Download a generated contact export (expires ~24h) |
| `POST /wa/webhook` | HMAC | Inbound WhatsApp messages + delivery/read receipts from the gateway (validated by `WA_WEBHOOK_SECRET`) |
| `GET /healthz` | public | Liveness probe (200 OK) |

Authenticate `/mcp` with **either** header:

```
Authorization: Bearer <MCP_API_KEY>
X-API-Key: <MCP_API_KEY>
```

The key check is constant-time and fail-closed. Tracking and export routes are intentionally public because email recipients open them without credentials.

## Features

- **CRM core:** contacts, imports/exports, bulk updates, pipeline stages, soft delete, and GDPR purge.
- **Email:** SMTP/Mailgun send, templates, merge rendering, unsubscribe compliance, open/click tracking, and email verification.
- **Campaigns:** segmented campaign creation, queued/background sends, scheduling, stats, and top-link analytics.
- **Inbox:** IMAP inbound reply sync, stored snippets/full bodies, read/unread, reply, and local delete.
- **WhatsApp:** two-way gateway integration, registration audit, smart-send throttling, inbound webhooks, read receipts, media lookup.
- **Threads:** hybrid Meta Threads channel with live publish/delete/search/replies/insights plus MySQL cache/audit and reply quota checks.
- **Ops:** health check, embedded migrations, CSV exports, Docker/EasyPanel-ready single-port deploy.

## Tools (63)

### Contacts
| Tool | Description |
|------|-------------|
| `contact_create` | Create a contact (only `email` required) |
| `contact_get` | Fetch a single contact by `id` or `email` |
| `contact_update` | Update a contact by `id` or `email` |
| `contact_list` | List contacts — paginated (limit default 20, cap 100), cursor, field projection |
| `contact_import` | Bulk import via array or CSV string; per-row error capture |
| `contact_export` | Export filtered contacts to a CSV download URL (not inline rows) |
| `contact_delete` | Soft-delete a contact; `purge: true` for a hard GDPR delete |
| `contact_unsubscribe` | Mark a contact unsubscribed (suppresses future email) |
| `contact_bulk_update` | Apply a partial patch to many contacts by ID list (max 500); tags via `add_tags`/`remove_tags` or `set_tags` |
| `contact_bulk_update_by_filter` | Apply a partial patch to every contact matching a segment filter (`stage`, `company`, `tag`, `q`) |
| `email_verify` | Verify one contact's email (syntax + DNS/MX + heuristics) and persist the verdict |
| `email_audit` | Batch-verify a segment of contacts; async by default (returns `task_id`), `sync:true` for a single inline page |

### Email & templates
| Tool | Description |
|------|-------------|
| `email_send` | Send one email to a contact or address (template or raw fields) |
| `template_create` | Create a reusable email template with merge variables |
| `template_get` | Fetch a template by `id` or `name` |
| `template_list` | List templates and their variables |
| `template_update` | Update a template |
| `template_delete` | Soft-delete a template |
| `template_render` | Render a template with vars without sending (text by default, HTML opt-in) |

### Campaigns
| Tool | Description |
|------|-------------|
| `campaign_create` | Create a campaign for a filtered contact segment |
| `campaign_get` | Fetch a campaign by id |
| `campaign_list` | List campaigns |
| `campaign_update` | Update a campaign (name, template, provider, segment, schedule) |
| `campaign_delete` | Soft-delete a campaign |
| `campaign_send` | Enqueue a campaign for background dispatch (returns `task_id`, status `queued`); `sync: true` sends inline and waits |
| `campaign_stats` | Delivery / open / click stats + top links for a campaign |

### Scheduling & tracking
| Tool | Description |
|------|-------------|
| `schedule_task` | Schedule an `email` or `campaign` task for future execution (RFC3339) |
| `task_list` | List scheduled tasks (filter by status) |
| `task_cancel` | Cancel a pending scheduled task |
| `tracking_link_create` | Wrap a URL in a click-tracked redirect |

### Inbox
| Tool | Description |
|------|-------------|
| `inbox_sync` | Manually fetch new inbound replies from configured IMAP mailbox |
| `inbox_list` | List stored inbound messages with snippets |
| `inbox_get` | Read full text/html body and headers for one inbound message |
| `inbox_mark_read` | Mark an inbound message read or unread |
| `inbox_reply` | Reply from the configured sender identity |
| `inbox_delete` | Soft-delete the local inbox copy; remote IMAP mail is not deleted |

### Threads
Hybrid Threads channel via Meta Threads Graph API. Enabled when `THREADS_ACCESS_TOKEN` is set. Optional env: `THREADS_USER_ID` (default `me`) and `THREADS_API_VERSION` (default `v1.0`). Live calls remain source of truth; posts/replies/mentions and audit events are cached in MySQL with raw Graph JSON for future API changes. Publishing supports official `topic_tag` (1-50 chars, disallow `.` and `&`); see `threads://publishing` resource.

| Tool | Description |
|------|-------------|
| `threads_profile` | Fetch configured Threads profile |
| `threads_profile_lookup` | Look up any public profile by username (name, bio, verified, follower_count + engagement counts); needs `threads_profile_discovery` |
| `threads_list` | List live Threads posts and cache them |
| `threads_publish` | Publish text/image/video post with optional `topic_tag` and cache result |
| `threads_delete` | Delete live post and soft-delete cached row |
| `threads_insights` | Fetch user-level or media-level insights |
| `threads_follower_demographics` | Fetch aggregate follower demographics (country/city/age/gender); requires ≥100 followers |
| `threads_replies` | List live replies for a post and cache them |
| `threads_reply_tree` | Nested reply conversation tree with `is_mine`/`needs_reply` flags |
| `threads_reply` | Reply to a Threads post or a specific comment (`reply_id`) |
| `threads_reply_hide` | Hide (moderate) a reply/comment on your post — no delete-comment API exists |
| `threads_reply_unhide` | Unhide a previously hidden reply on your post |
| `threads_reply_quota` | Fetch reply publishing quota usage |
| `threads_mentions` | List live mentions and cache them |
| `threads_search` | Keyword/topic search via Threads API with TOP/RECENT, KEYWORD/TAG, media type, author, time range, and fields |
| `threads_discover` | Cookie-only discovery of PUBLIC posts by topic (no token; separate x-threads-utils path). `mode=posts` returns structured JSON (pk, code, username, caption, like_count); `mode=viral`/`latest` return engagement-ranked or newest-first text. For AI-agent research/training |
| `threads_token_exchange` | Exchange a short-lived Threads token for a long-lived token |
| `threads_token_refresh` | Refresh a long-lived Threads token before expiry |
| `threads_list_cached` | List cached Threads posts |
| `threads_get_cached` | Get cached Threads post by local id or Threads id |
| `threads_history` | List Threads audit events |
| `threads_delete_cached` | Soft-delete cached row only |

### WhatsApp
Two-way WhatsApp channel via a [go-whatsapp-web-multidevice](https://github.com/aldinokemal/go-whatsapp-web-multidevice) gateway. Enabled only when `WA_BASE_URL` + `WA_DEVICE_ID` are set. Outbound uses WhatsApp markdown (`*bold*`, `_italic_`, `~strike~`, ` ```code``` `) — see the `whatsapp://formatting` MCP resource. Inbound replies and receipts arrive on `POST /wa/webhook`.

| Tool | Description |
|------|-------------|
| `whatsapp_check` | Check if a contact (`id`) or `phone` is registered on WhatsApp; persists the verdict |
| `whatsapp_audit` | Batch-check a contact segment for WhatsApp registration (skips already-checked by default) |
| `whatsapp_send` | Send a WhatsApp message to a contact or phone; smart-send throttling + unregistered-block applied |
| `whatsapp_list` | List stored WhatsApp messages (filter by direction/phone), paginated |
| `whatsapp_get` | Read one stored WhatsApp message |
| `whatsapp_reply` | Reply to an inbound message (threads via quoted message id, marks it replied) |
| `whatsapp_mark_read` | Mark an inbound WhatsApp message read on the gateway |
| `whatsapp_get_media` | Fetch media (proxy URL + mime, optional inline base64) for a message with an attachment |

### Ops & analytics
| Tool | Description |
|------|-------------|
| `health_check` | Self-test DB and email connectivity |
| `analytics_overview` | High-level CRM + communication metrics |

### Token optimization

Responses are kept small for agent context budgets:

- `contact_list` paginates, projects requested `fields`, and returns a compact envelope `{total, count, items, next_cursor}`.
- `contact_export` returns a download URL + row count, never inline rows.
- `template_render` returns subject + text by default; HTML only when `html: true`.
- Errors are terse: `{error, msg}`. Raw infrastructure errors are not leaked to clients.

## Pipeline stages

Fixed enum: `new → contacted → qualified → proposal → won → lost`. Invalid stages are rejected.

## Compliance & data lifecycle

- **Unsubscribe.** Every contact has a public opt-out token. Contact-addressed emails carry an unsubscribe footer linking to `GET /u/{code}`, plus `List-Unsubscribe` and `List-Unsubscribe-Post: List-Unsubscribe=One-Click` headers (RFC 2369 / 8058) so Gmail and Yahoo surface a native unsubscribe button. Mail clients POST to `POST /u/{code}` for one-click opt-out. Visiting or posting sets `unsubscribed_at` and logs an `unsubscribe` event. `email_send` and `campaign_send` refuse to send to unsubscribed contacts (campaigns count them as `skipped`).
- **Soft delete.** `contact_delete`, `campaign_delete`, and `template_delete` set `deleted_at`; deleted rows are hidden from all lists, sends, and stats but remain recoverable.
- **Hard delete (GDPR).** `contact_delete` with `purge: true` removes the row permanently.

## Architecture

Zetta CRM uses a hexagonal (ports & adapters) layout — `domain → port → service`, with `adapter/*` (MySQL, email, system) and `transport/*` (MCP, HTTP) on the edges. The service layer holds all business logic and is tested without a database. See [ARCHITECTURE.md](ARCHITECTURE.md) for the dependency rule, package map, and how to add a tool or swap an adapter.

Install guides live in the [GitHub Wiki](https://github.com/incredible-zetta/crm/wiki) and are mirrored in [`docs/wiki/`](docs/wiki/).

## Configuration

All config comes from environment variables (EasyPanel injects them). See `.env.example`.

| Variable | Required | Default | Notes |
|----------|----------|---------|-------|
| `MCP_API_KEY` | yes | — | Bearer / X-API-Key value agents must send |
| `DB_DSN` | yes | — | MySQL DSN, e.g. `user:pass@tcp(host:3306)/crmagents?parseTime=true&multiStatements=true` |
| `BASE_URL` | yes | — | Public base URL, used to build tracking + export links |
| `PORT` | no | `8080` | Listen port |
| `SCHEDULER_INTERVAL_SEC` | no | `15` | Scheduler tick interval (seconds) |
| `EXPORT_DIR` | no | `/data/exports` | Directory for generated CSV files |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_USER` / `SMTP_PASS` / `SMTP_FROM` | no | — | SMTP sender config |
| `MAILGUN_DOMAIN` / `MAILGUN_API_KEY` | no | — | Mailgun sender config (preferred when both set) |
| `LOG_LEVEL` | no | — | Set to `debug` to print startup diagnostics and request logs to container stderr/stdout |
| `IMAP_HOST` / `IMAP_PORT` / `IMAP_USER` / `IMAP_PASS` / `IMAP_MAILBOX` | no | port `993`, mailbox `INBOX` | Optional inbound mailbox for lead replies |
| `IMAP_POLL_INTERVAL_SEC` | no | `60` | Inbox polling interval when IMAP is enabled |
| `IMAP_SINCE_DAYS` | no | `14` | First-sync lookback window |
| `ADMIN_NOTIFY_EMAIL` | no | — | Admin email notified for new replies from known contacts |
| `EMAIL_RATE_MAX` / `EMAIL_RATE_WINDOW_SEC` | no | — | Throttle outbound email to `MAX` sends per `WINDOW` seconds (e.g. `200` / `100` for Larksuite). Both must be set to enable. |
| `VERIFY_EMAILS` | no | `false` | Verify email on contact create/update (syntax + DNS/MX + disposable/role heuristics) and persist the verdict |
| `BLOCK_INVALID_SEND` | no | `false` | Refuse to send to contacts whose persisted email status is `invalid` |
| `THREADS_ACCESS_TOKEN` | no | — | Threads Graph API token. Set to enable Threads channel; never log/commit it |
| `THREADS_APP_SECRET` | no | — | Threads app secret, required only for `threads_token_exchange`; never log/commit it |
| `THREADS_USER_ID` | no | `me` | Threads user ID for profile/list/publish/reply endpoints |
| `THREADS_API_VERSION` | no | `v1.0` | Threads Graph API version |
| `THREADS_DISCOVERY_BIN` | no | — | Path to the [x-threads-utils](https://github.com/incredible-zetta/x-threads-utils/releases) `threads` binary. With `THREADS_COOKIES_FILE`, enables the cookie-only `threads_discover` tool (no token) |
| `THREADS_COOKIES_FILE` | no | — | Path to a Netscape cookie-file export of a logged-in threads.com session, used by `threads_discover`. Holds a live session; never commit it |
| `WA_BASE_URL` | no | — | WhatsApp gateway base URL. Set (with `WA_DEVICE_ID`) to enable the WhatsApp channel |
| `WA_BASIC_AUTH` | no | — | Raw base64 of `user:pass` for gateway Basic Auth (never logged) |
| `WA_DEVICE_ID` | no | `cds` | `x-device-id` header sent to the gateway |
| `WA_WEBHOOK_SECRET` | no | — | HMAC-SHA256 secret validating `POST /wa/webhook` (empty = no validation; set it in prod) |
| `WA_SEND_MAX` / `WA_SEND_WINDOW_SEC` | no | — | Token-bucket throttle: max sends per window (ban-prevention) |
| `WA_SEND_DAILY_CAP` | no | — | Per-recipient daily send cap |
| `WA_SEND_JITTER_MIN_MS` / `WA_SEND_JITTER_MAX_MS` | no | — | Random delay range before each send (human-like pacing) |
| `WA_WARMUP_PER_DAY` | no | — | Global 24h send ceiling (number warm-up) |
| `WA_BLOCK_UNREGISTERED_SEND` | no | `false` | Refuse sends to numbers verified NOT on WhatsApp |

Provider selection: Mailgun is used when both `MAILGUN_DOMAIN` and `MAILGUN_API_KEY` are set, otherwise SMTP. If neither is configured the server still boots with a disabled sender that fails explicitly at send time (`email_send`/`campaign_send` return a terse error).

Debug logging: set `LOG_LEVEL=debug` to print redacted startup diagnostics, route registration, and HTTP request logs to the container logs.

Inbox: IMAP polling is disabled unless `IMAP_HOST`, `IMAP_USER`, `IMAP_PASS`, `IMAP_MAILBOX`, and `ADMIN_NOTIFY_EMAIL` are set. New inbound messages are stored, and admin notifications are sent only for known contacts matched by sender email.

Email rate limiting: set `EMAIL_RATE_MAX` and `EMAIL_RATE_WINDOW_SEC` to pace outbound delivery under a provider cap (Larksuite allows 200 messages / 100s, so `EMAIL_RATE_MAX=200` and `EMAIL_RATE_WINDOW_SEC=100`). The limiter is a token bucket applied to every send — single `email_send` and `campaign_send` alike — so a campaign loop blocks until a slot frees instead of bursting. Leave unset to send without throttling.

Threads: the channel is disabled unless `THREADS_ACCESS_TOKEN` is set. Required scopes depend on the tool: `threads_basic`, `threads_content_publish`, `threads_manage_replies`, `threads_read_replies`, `threads_manage_insights`, `threads_manage_mentions`, `threads_keyword_search`, `threads_profile_discovery`, and `threads_delete` cover the full tool set. Reply publishing uses Meta's two-step flow: create a reply container via `POST /{user_id}/threads` with `reply_to_id`, then publish via `POST /{user_id}/threads_publish`. Posts and replies cannot be edited (no update endpoint); to change content, delete and re-create. There is no API to delete another user's comment on your post — moderate unwanted replies with `threads_reply_hide`/`threads_reply_unhide` (`POST /{reply_id}/manage_reply`), which needs `threads_manage_replies` and only works on replies under your own posts. Search supports `TOP`/`RECENT`, `KEYWORD`/`TAG`, media type, author username, time range, and fields; public search may require approved `threads_keyword_search`, otherwise results can be limited to authenticated user's posts. Graph-generated short-lived tokens can be exchanged with `threads_token_exchange` and long-lived tokens refreshed with `threads_token_refresh`. Check `threads_reply_quota` before automation; the API reports `reply_quota_usage` and `reply_config`. See [docs/wiki/Threads-Channel.md](docs/wiki/Threads-Channel.md) for examples and live verification notes.

Threads discovery (cookie-only): the `threads_discover` tool is a **separate path** from the Graph API above. It reads PUBLIC posts of any account for AI-agent research/training using a logged-in browser session cookie — no access token. Enabled only when both `THREADS_DISCOVERY_BIN` (the [x-threads-utils](https://github.com/incredible-zetta/x-threads-utils/releases) binary) and `THREADS_COOKIES_FILE` are set. `mode=posts` returns structured JSON (`pk`, `code`, `username`, `caption`, `like_count`); `mode=viral`/`latest` return engagement-ranked or newest-first text. IDs returned (`pk`, `code`) are web IDs and are **not** valid on the Graph API — only the `username` bridges the two paths, and validating a discovered username via `threads_profile_lookup` requires Meta-approved `threads_profile_discovery` (currently gated for arbitrary handles). The cookie file holds a live session; never commit it.

WhatsApp: the channel is disabled unless `WA_BASE_URL` and `WA_DEVICE_ID` are set. When enabled, outbound sends pass through a smart-send wrapper (token-bucket window, per-recipient daily cap, random jitter, global warm-up ceiling) to reduce ban risk — all caps default to off, set them for real numbers. `whatsapp_check`/`whatsapp_audit` persist registration verdicts; with `WA_BLOCK_UNREGISTERED_SEND=true`, `whatsapp_send` refuses numbers known not on WhatsApp. Inbound messages and delivery/read receipts arrive at `POST /wa/webhook`, validated with HMAC-SHA256 when `WA_WEBHOOK_SECRET` is set (incoming messages are linked to known contacts by phone). See [docs/wiki/WhatsApp-Channel.md](docs/wiki/WhatsApp-Channel.md) for the full guide.

Email verification: set `VERIFY_EMAILS=true` to verify addresses on contact create/update. Verification is self-hosted — RFC syntax check, DNS MX lookup (with A/AAAA fallback), and disposable/role-address heuristics — and records a verdict per contact: `valid`, `invalid`, `risky`, or `unknown`. It deliberately skips SMTP RCPT probing, which is unreliable and harms sender reputation, so `valid` means deliverable-capable, not guaranteed-inbox. Use `email_verify` for one contact and `email_audit` to sweep a segment (async by default). Set `BLOCK_INVALID_SEND=true` to refuse sending to contacts verified `invalid`.

> If the DSN password contains shell metacharacters (e.g. `!`), single-quote the value.

## Database

MySQL 8. Migrations are embedded in the binary and applied automatically on startup (`golang-migrate`). Tables: `contacts`, `email_templates`, `campaigns`, `tracking_links`, `email_events`, `scheduled_tasks`, `exports`, `inbound_messages`, `inbox_cursors`, `wa_messages`, `threads_posts`, `threads_replies`, `threads_mentions`, `threads_audit_events` (+ `schema_migrations`). `contacts` carries WhatsApp registration columns (`whatsapp_status`, `whatsapp_checked_at`). Contacts, campaigns, and templates carry a `deleted_at` for soft delete; contacts also carry `unsubscribed_at` + a public `unsub_code`. Threads rows keep typed columns plus `raw_json` for Graph API drift.

## Run locally

```bash
cp .env.example .env   # fill in values
export $(grep -v '^#' .env | xargs)   # or use a dotenv loader
go run ./cmd/server
```

Then smoke-test:

```bash
MCP_URL=http://localhost:8080/mcp MCP_API_KEY=<your-key> ./scripts/test-mcp.sh
```

The script runs `initialize → notifications/initialized → tools/list → tools/call health_check` and prints the responses.

## Test & quality checks

```bash
make fmt        # rewrite Go files with gofmt
make check-fmt  # fail when gofmt would change files
make lint       # go vet + staticcheck when installed
make test       # go test ./...
make check      # check-fmt + lint + test + build
```

Staticcheck is optional locally but required in CI. Install it with:

```bash
make staticcheck
```

Integration tests need `DB_DSN` (else they skip):

```bash
export DB_DSN='user:pass@tcp(localhost:3306)/crmagents?parseTime=true&multiStatements=true'
go test ./...
```

## Build the image

```bash
docker build -t crm-mcp .
# or pull release image: ghcr.io/incredible-zetta/crm:v0.0.1-beta
docker run --rm -p 8080:8080 \
  -e MCP_API_KEY=... -e DB_DSN=... -e BASE_URL=https://crm.example.com \
  -v crm-exports:/data/exports \
  crm-mcp
```

Multi-stage build: `golang:1.25` → `distroless/static` (~14MB, runs as non-root).

## Deploy on EasyPanel

1. Create an app from this repo's `Dockerfile` (no compose needed).
2. Provision/point to a MySQL 8 database and set `DB_DSN`.
3. Set env vars: `MCP_API_KEY` (long random), `BASE_URL` (the app's public URL), `DB_DSN`, and email provider vars (SMTP or Mailgun).
4. Expose the container port (`8080`) on your domain. One domain serves both `/mcp` and the public tracking/export routes — so `BASE_URL` must equal that public URL for tracking links to resolve.
5. Mount a persistent volume at `EXPORT_DIR` (`/data/exports`) if you want export downloads to survive restarts.
6. Health check path: `/healthz`.

Migrations run on boot, so the first start initializes the schema automatically.

### Connecting an agent

Point any MCP client at `https://<your-domain>/mcp` using Streamable HTTP transport and send the API key header. Example client config:

```json
{
  "mcpServers": {
    "zettacrm": {
      "type": "streamable-http",
      "url": "https://crm.your-domain.com/mcp",
      "headers": { "Authorization": "Bearer <MCP_API_KEY>" }
    }
  }
}
```

## License & credits

### Project layout

```
cmd/server/                 composition root: wire adapters -> services -> transports
internal/domain/            entities, enums, sentinel errors (pure, no internal deps)
internal/port/              repository / sender / clock / idgen interfaces
internal/service/           use-case layer: all business logic (tested without a DB)
internal/adapter/mysql/     repositories implementing the ports against MySQL
internal/adapter/email/     SMTP + Mailgun senders
internal/adapter/system/    real clock + crypto id generator
internal/template/          render + link rewrite + open pixel + unsubscribe footer
internal/scheduler/         in-process worker (claim -> execute -> mark)
internal/mcpserver/         MCP server scaffold + auth + terse response helpers
internal/adapter/whatsapp/  WhatsApp gateway client, phone normalize, WA-markdown, smart-send wrapper
internal/transport/mcp/     46 thin MCP tool handlers
internal/transport/http/    public routes (click, open pixel, export, unsubscribe, health, WA webhook)
internal/config/            env config loader
migrations/                 embedded schema: 0001_init, 0002_inbox, ... 0005_whatsapp
scripts/                    test-mcp.sh smoke test
docs/plans/                 design + implementation plans
ARCHITECTURE.md             the hexagon, dependency rule, how to extend
```

## Credits

Zetta CRM is built and maintained by [Incredible Zetta](https://github.com/incredible-zetta) in partnership with [Ciptadusa (CDS)](https://github.com/cds-id).
