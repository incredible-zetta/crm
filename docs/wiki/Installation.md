# Installation

Zetta CRM runs as one container plus one MySQL 8 database.

Published images are pushed to GitHub Container Registry:

```bash
# release (production)
ghcr.io/incredible-zetta/crm:v0.0.1-beta

# master pre-release (auto-built on every merge to master)
ghcr.io/incredible-zetta/crm:master
ghcr.io/incredible-zetta/crm:master-<short-sha>
```

Use release tags in production. Use `master` or `master-<short-sha>` to test changes before a tagged release.

## Required environment

| Variable | Required | Notes |
|----------|----------|-------|
| `MCP_API_KEY` | yes | Bearer / `X-API-Key` for `/mcp` |
| `DB_DSN` | yes | MySQL DSN: `user:pass@tcp(host:3306)/crmagents?parseTime=true&multiStatements=true` |
| `BASE_URL` | yes | Public URL used for tracking, export, unsubscribe links |
| `PORT` | no | Default `8080` |
| `EXPORT_DIR` | no | Default `/data/exports` in image |
| `SCHEDULER_INTERVAL_SEC` | no | Default `15` |
| `SMTP_*` or `MAILGUN_*` | no | Needed for real email sending |

SMTP example:

```bash
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=no-reply@example.com
SMTP_PASS=...
SMTP_FROM=no-reply@example.com
```

Mailgun example:

```bash
MAILGUN_DOMAIN=mg.example.com
MAILGUN_API_KEY=...
SMTP_FROM=no-reply@example.com
```

## Local Docker

```bash
docker network create zetta || true

docker run -d --name zetta-mysql --network zetta \
  -e MYSQL_ROOT_PASSWORD=rootpass \
  -e MYSQL_DATABASE=crmagents \
  -e MYSQL_USER=crmagents \
  -e MYSQL_PASSWORD=crmagentspass \
  -v zetta-mysql:/var/lib/mysql \
  mysql:8

docker run -d --name zetta-crm --network zetta \
  -p 8080:8080 \
  -e MCP_API_KEY='change-me-long-random' \
  -e BASE_URL='http://localhost:8080' \
  -e DB_DSN='crmagents:crmagentspass@tcp(zetta-mysql:3306)/crmagents?parseTime=true&multiStatements=true' \
  -v zetta-exports:/data/exports \
  ghcr.io/incredible-zetta/crm:v0.0.1-beta
```

Smoke test:

```bash
curl -fsS http://localhost:8080/healthz
MCP_URL=http://localhost:8080/mcp MCP_API_KEY=change-me-long-random ./scripts/test-mcp.sh
```

## Docker Compose

```yaml
services:
  mysql:
    image: mysql:8
    environment:
      MYSQL_ROOT_PASSWORD: rootpass
      MYSQL_DATABASE: crmagents
      MYSQL_USER: crmagents
      MYSQL_PASSWORD: crmagentspass
    volumes:
      - mysql:/var/lib/mysql

  zetta-crm:
    image: ghcr.io/incredible-zetta/crm:v0.0.1-beta
    depends_on:
      - mysql
    ports:
      - "8080:8080"
    environment:
      MCP_API_KEY: change-me-long-random
      BASE_URL: http://localhost:8080
      DB_DSN: crmagents:crmagentspass@tcp(mysql:3306)/crmagents?parseTime=true&multiStatements=true
      EXPORT_DIR: /data/exports
    volumes:
      - exports:/data/exports

volumes:
  mysql:
  exports:
```

## Reverse proxy

Public routes:

- `/mcp` — private; require `Authorization: Bearer $MCP_API_KEY` or `X-API-Key`.
- `/healthz` — public health check.
- `/t/{code}` — public click redirect.
- `/o/{code}.png` — public open pixel.
- `/export/{id}.csv` — public export download URL.
- `/u/{code}` — public unsubscribe page.

Terminate TLS at your platform/proxy and set `BASE_URL=https://your-domain.example`.

## Optional IMAP inbox

Set these env vars to enable inbound reply sync and admin notifications:

```env
IMAP_HOST=imap.example.com
IMAP_PORT=993
IMAP_USER=no-reply@example.com
IMAP_PASS=your-imap-password
IMAP_MAILBOX=INBOX
IMAP_POLL_INTERVAL_SEC=60
IMAP_SINCE_DAYS=14
ADMIN_NOTIFY_EMAIL=admin@example.com
```

When enabled, Zetta CRM polls IMAP, stores inbound replies, matches known contacts by sender email, and exposes `inbox_sync`, `inbox_list`, `inbox_get`, `inbox_mark_read`, `inbox_reply`, and `inbox_delete` over MCP.
