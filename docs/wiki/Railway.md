# Railway install

Railway can run Zetta CRM as a Docker image plus a MySQL plugin/service.

## 1. Add MySQL

Create a MySQL database. Use Railway private networking when possible.

Build DSN:

```text
DB_DSN=user:password@tcp(host:3306)/railway?parseTime=true&multiStatements=true
```

Use your actual database name.

## 2. Add service from image

Image:

```text
ghcr.io/incredible-zetta/crm:v0.0.1-beta
```

Railway injects `PORT`; Zetta CRM reads `PORT`, defaulting to `8080` if absent.

## 3. Variables

```text
MCP_API_KEY=<long random secret>
BASE_URL=https://<your-service>.up.railway.app
DB_DSN=<mysql dsn>
EXPORT_DIR=/data/exports
```

Email variables as needed:

```text
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=no-reply@example.com
SMTP_PASS=<password>
SMTP_FROM=no-reply@example.com
```

## 4. Volumes

If you use `contact_export`, attach a volume at:

```text
/data/exports
```

Without a volume, exports are ephemeral.

## 5. Verify

```bash
curl -fsS https://<your-service>.up.railway.app/healthz
```
