# Coolify install

## App

Create a Docker image app:

```text
ghcr.io/incredible-zetta/crm:v0.0.1-beta
```

Port:

```text
8080
```

Domain example:

```text
https://crm.example.com
```

Set `BASE_URL` to the same public URL.

## Database

Create a MySQL 8 resource in Coolify, then build:

```text
DB_DSN=user:password@tcp(mysql-host:3306)/crmagents?parseTime=true&multiStatements=true
```

Use the internal service hostname shown by Coolify, not `localhost`.

## Required env

```text
MCP_API_KEY=<long random secret>
BASE_URL=https://crm.example.com
DB_DSN=<mysql dsn>
EXPORT_DIR=/data/exports
```

## Storage

Add persistent volume:

```text
/data/exports
```

## Health check

```text
GET /healthz
```

## Notes

- `/mcp` must be reached over HTTPS for most hosted agent clients.
- Keep `MCP_API_KEY` private.
- Do not expose MySQL publicly unless your platform requires it; prefer internal network hostname.
