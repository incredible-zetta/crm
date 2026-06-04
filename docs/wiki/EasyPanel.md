# EasyPanel install

## 1. Create MySQL 8 database

In EasyPanel, create a MySQL service. Note host, database, username, and password.

Use DSN format:

```text
user:password@tcp(mysql-host:3306)/crmagents?parseTime=true&multiStatements=true
```

If password has `!`, `&`, `(`, `)`, or spaces, paste as a literal env value in EasyPanel UI; do not shell-export it unquoted.

## 2. Create app

Create an app from image:

```text
ghcr.io/incredible-zetta/crm:v0.0.1-beta
```

Expose port:

```text
8080
```

## 3. Environment variables

```text
MCP_API_KEY=<long random secret>
BASE_URL=https://crm.example.com
DB_DSN=<mysql dsn>
EXPORT_DIR=/data/exports
SCHEDULER_INTERVAL_SEC=15
```

Email via SMTP:

```text
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=no-reply@example.com
SMTP_PASS=<password>
SMTP_FROM=no-reply@example.com
```

or Mailgun:

```text
MAILGUN_DOMAIN=mg.example.com
MAILGUN_API_KEY=<key>
SMTP_FROM=no-reply@example.com
```

## 4. Persistent storage

Mount a volume at:

```text
/data/exports
```

Exports are CSV files returned by `contact_export`.

## 5. Health check

Use:

```text
/healthz
```

Expected: HTTP 200 with `ok`.

## 6. Agent configuration

MCP URL:

```text
https://crm.example.com/mcp
```

Auth:

```text
Authorization: Bearer <MCP_API_KEY>
```

or:

```text
X-API-Key: <MCP_API_KEY>
```
