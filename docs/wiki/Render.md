# Render install

Render supports Docker web services. Use an external MySQL provider (PlanetScale, Aiven, DigitalOcean Managed MySQL, AWS RDS, etc.) because Render's managed DB is PostgreSQL.

## Web Service

Create **New Web Service → Existing image**:

```text
ghcr.io/incredible-zetta/crm:v0.0.1-beta
```

Port:

```text
8080
```

Render may inject `PORT`; Zetta CRM supports that.

## Environment

```text
MCP_API_KEY=<long random secret>
BASE_URL=https://crm.example.onrender.com
DB_DSN=user:password@tcp(mysql-host:3306)/crmagents?parseTime=true&multiStatements=true
EXPORT_DIR=/data/exports
```

## Disk

Add persistent disk mounted at:

```text
/data/exports
```

## Health check

```text
/healthz
```

## MySQL TLS

If your MySQL provider requires TLS, add DSN params supported by Go MySQL driver. Example provider-specific DSNs vary; keep `parseTime=true&multiStatements=true`.
