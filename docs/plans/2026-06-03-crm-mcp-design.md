# CRM-for-AI-Agents — MCP Service Design

Date: 2026-06-03
Status: Approved (brainstorm complete)

## 1. Purpose

A CRM backend exposed entirely as an **MCP (Model Context Protocol) server** so any
AI Agent can manage contacts, run email marketing campaigns, track engagement
(opens/clicks), schedule sends, and read analytics. Single Go binary, MySQL store,
deployed to a VPS via **EasyPanel using a Dockerfile only**. Access gated by a
hardcoded API key in `.env`.

## 2. Tech Stack

| Concern        | Choice                                                        |
|----------------|--------------------------------------------------------------|
| Language       | Go (1.23+)                                                    |
| MCP SDK        | `github.com/modelcontextprotocol/go-sdk` (v1.4.0+)           |
| Transport      | Streamable HTTP (`StreamableHTTPHandler`)                     |
| DB             | MySQL 8.0 (`database/sql` + `github.com/go-sql-driver/mysql`) |
| Migrations     | `golang-migrate` embedded, run on boot                       |
| Email          | SMTP (`net/smtp` / `go-mail`) + Mailgun HTTP API              |
| Templating     | `html/template` + `text/template`                            |
| Scheduler      | In-process goroutine ticker, MySQL-backed job table          |
| Deploy         | Single Dockerfile, EasyPanel, one port, one domain           |

## 3. Network / Routing

One service, one HTTP listener, two route groups:

| Route            | Auth         | Purpose                                            |
|------------------|--------------|----------------------------------------------------|
| `POST /mcp`      | API key      | MCP Streamable HTTP endpoint (all 16 tools)        |
| `GET /t/{code}`  | public       | Click tracking → 302 redirect to real URL, log click |
| `GET /o/{code}.png` | public    | Open tracking → 1x1 gif pixel, log open            |
| `GET /export/{id}.csv` | token   | Export file download (token-optimized exports)     |
| `GET /healthz`   | public       | Liveness probe for EasyPanel                        |

### Auth model
`NewStreamableHTTPHandler(getServer)` — the `getServer` func inspects the request
`Authorization: Bearer <MCP_API_KEY>` (or `X-API-Key`). Wrong/missing key → 401
before any MCP session is created. Tracking routes are intentionally public
(emails are opened by arbitrary recipients).

## 4. Data Model (MySQL)

```sql
contacts (
  id BIGINT PK AUTO_INCREMENT,
  email VARCHAR(320) UNIQUE NOT NULL,
  first_name VARCHAR(120), last_name VARCHAR(120),
  company VARCHAR(200), phone VARCHAR(40),
  stage ENUM('new','contacted','qualified','proposal','won','lost') DEFAULT 'new',
  tags JSON, notes TEXT, custom JSON,
  source VARCHAR(80),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX(stage), INDEX(company)
)

email_templates (
  id BIGINT PK, name VARCHAR(160) UNIQUE,
  subject VARCHAR(400), body_html MEDIUMTEXT, body_text MEDIUMTEXT,
  variables JSON, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)

campaigns (
  id BIGINT PK, name VARCHAR(200),
  template_id BIGINT, provider ENUM('smtp','mailgun'),
  segment JSON,                 -- filter: stage/tags/company
  status ENUM('draft','scheduled','sending','sent','failed') DEFAULT 'draft',
  scheduled_at TIMESTAMP NULL,
  stats JSON,                   -- cached aggregate
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)

tracking_links (
  id BIGINT PK, code CHAR(12) UNIQUE,
  target_url TEXT, campaign_id BIGINT NULL, contact_id BIGINT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)

email_events (
  id BIGINT PK,
  contact_id BIGINT, campaign_id BIGINT NULL,
  type ENUM('sent','delivered','open','click','bounce','failed'),
  link_code CHAR(12) NULL, meta JSON, ts TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  INDEX(campaign_id, type), INDEX(contact_id)
)

scheduled_tasks (
  id BIGINT PK,
  kind ENUM('email','campaign'),
  payload JSON,                 -- template_id, contact_id/segment, provider
  run_at TIMESTAMP, status ENUM('pending','running','done','failed') DEFAULT 'pending',
  attempts INT DEFAULT 0, last_error TEXT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  INDEX(status, run_at)
)

exports (
  id CHAR(16) PK, path VARCHAR(300), rows INT,
  expires_at TIMESTAMP, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)
```

## 5. MCP Tools (16)

Contacts
1. `contact_create` — upsert by email. In: email, names, company, stage, tags, custom.
2. `contact_update` — patch by id/email. Validates stage enum.
3. `contact_list` — filter(stage,tag,company,q) + **pagination** (limit≤100, cursor) + **fields** projection. Returns `{total,count,items,next_cursor}`.
4. `contact_import` — bulk array OR CSV text. Returns `{inserted,updated,skipped,errors[]}`.
5. `contact_export` — builds CSV file, returns **URL** `/export/{id}.csv` (NOT inline) + row count.

Email & Campaigns
6. `email_send` — single. In: contact_id/email, template_id|raw subject/body, provider(smtp|mailgun). Injects tracking pixel + rewrites links. Logs `sent`.
7. `campaign_create` — name, template_id, segment, provider, optional scheduled_at.
8. `campaign_send` — send now (or enqueue if scheduled). Expands segment, per-contact render.

Templates
9. `template_create` — name, subject, body_html, body_text, variables.
10. `template_list` — names + variables (compact).
11. `template_render` — preview render w/ sample vars. Returns `body_text` by default; `body_html` only if `html:true`. **No send.**

Tracking
12. `tracking_link_create` — wrap target_url → returns short `/t/{code}` URL.

Scheduler
13. `schedule_task` — kind(email|campaign), payload, run_at. Returns task id.

Ops & Analytics
14. `health_check` — `{status,db_ok,smtp_ok,mailgun_ok,version,time}`.
15. `analytics_overview` — flat: contacts by stage, total, campaigns_sent, opens, clicks, open_rate, click_rate, pending_tasks.
16. `campaign_stats` — per campaign: sent, delivered, opened, clicked, bounced, open_rate, click_rate, top_links[].

## 6. Token Optimization Rules

- Compact JSON (no indent). Minimal default fields.
- List/export tools paginate (limit default 20, max 100) + cursor.
- `fields` projection param on list tools.
- Summary-first envelope `{total,count,items,next_cursor}`.
- `contact_export` returns a **download URL**, never inline rows.
- `template_render` defaults to text body; HTML opt-in.
- Errors terse: `{error:"<code>", msg:"<short>"}`. No stack traces over MCP.
- Analytics return flat number maps, no deep nesting.

## 7. Email Sending Pipeline

1. Resolve template (or raw) → render with contact vars (`html/template`).
2. Rewrite every `<a href>` → `/t/{code}` (create `tracking_links`, log on click).
3. Append open pixel `<img src="/o/{code}.png">`.
4. Send via chosen provider (SMTP or Mailgun API).
5. Insert `email_events` row type `sent` (+ `failed` on error).
6. Provider webhooks (Mailgun) optional later → `delivered`/`bounce`.

## 8. Scheduler

Boot starts one goroutine: every 15s `SELECT ... WHERE status='pending' AND run_at<=NOW() LIMIT N FOR UPDATE SKIP LOCKED`, mark `running`, execute, mark `done`/`failed` (retry w/ attempts cap). State in MySQL → restart-safe. Single container, no Redis.

## 9. Project Structure

```
crm-for-aiagents/
├── cmd/server/main.go            # wire config, db, mcp, http, scheduler
├── internal/
│   ├── config/                   # env load (MCP_API_KEY, DB, SMTP, MAILGUN, BASE_URL)
│   ├── db/                       # sql conn, migrations (embed), queries
│   ├── mcp/                      # server setup + auth + tool registration
│   │   └── tools/                # one file per tool group
│   ├── email/                    # smtp.go, mailgun.go, sender.go (iface)
│   ├── template/                 # render + link-rewrite + pixel
│   ├── tracking/                 # /t /o handlers + code gen
│   ├── scheduler/                # ticker worker
│   ├── export/                   # csv build + /export handler
│   └── analytics/                # aggregate queries
├── migrations/                   # *.up.sql / *.down.sql (embedded)
├── scripts/test-mcp.sh           # curl initialize + tools/call health_check
├── .env.example
├── Dockerfile                    # multi-stage, scratch/distroless final
└── docs/plans/2026-06-03-crm-mcp-design.md
```

## 10. Config (.env)

```
MCP_API_KEY=          # hardcoded bearer key agents must send
BASE_URL=             # https://crm.example.com (for tracking links)
DB_DSN=               # crmagents_user:pass@tcp(host:3306)/crmagents?parseTime=true
SMTP_HOST= SMTP_PORT= SMTP_USER= SMTP_PASS= SMTP_FROM=
MAILGUN_DOMAIN= MAILGUN_API_KEY=
PORT=8080
SCHEDULER_INTERVAL_SEC=15
```

## 11. Dockerfile (multi-stage)

Stage 1 `golang:1.23` build static binary. Stage 2 `gcr.io/distroless/static` copy binary + migrations embedded. Expose `8080`. `ENTRYPOINT ["/crm-server"]`. EasyPanel maps domain → port 8080, injects env vars.

## 12. Testing

- `scripts/test-mcp.sh`: `initialize` → `tools/list` → `tools/call health_check`.
- `template_render` MCP tool to eyeball templates without sending.
- Unit tests: template render+link-rewrite, segment expansion, scheduler due-selection, analytics aggregates.
- Local MySQL via docker-compose (dev only, not shipped).

## 13. Out of Scope (YAGNI)

OAuth, multi-tenant, web UI, Redis queue, separate tracking service, configurable
pipelines, inbound email parsing. Revisit only if needed.
```
