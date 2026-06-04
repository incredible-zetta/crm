# Google Cloud Run install

Cloud Run can run Zetta CRM as a container. Use Cloud SQL for MySQL or another managed MySQL 8 provider.

## Image

```text
ghcr.io/incredible-zetta/crm:v0.0.1-beta
```

Cloud Run uses `PORT`; Zetta CRM reads it.

## Cloud SQL MySQL

Create Cloud SQL MySQL 8. You can connect through public/private IP depending on your setup.

TCP DSN example:

```text
DB_DSN=user:password@tcp(private-ip:3306)/crmagents?parseTime=true&multiStatements=true
```

If you need Cloud SQL Auth Proxy/socket style, deploy a sidecar or use your platform's connector pattern. Keep DSN compatible with `github.com/go-sql-driver/mysql`.

## Service variables

```text
MCP_API_KEY=<secret>
BASE_URL=https://crm.example.com
DB_DSN=<mysql dsn>
EXPORT_DIR=/data/exports
```

## Exports

Cloud Run filesystem is ephemeral. For durable exports, mount Cloud Storage FUSE or use a persistent file strategy. MVP image writes CSVs to local `EXPORT_DIR`; without persistence, exports disappear when instance is replaced.

## Health check

Use `/healthz`.

## Deploy example

```bash
gcloud run deploy zetta-crm \
  --image ghcr.io/incredible-zetta/crm:v0.0.1-beta \
  --allow-unauthenticated \
  --set-env-vars BASE_URL=https://crm.example.com,EXPORT_DIR=/data/exports \
  --set-secrets MCP_API_KEY=zettacrm-mcp-key:latest,DB_DSN=zettacrm-db-dsn:latest
```
