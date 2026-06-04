# Fly.io install

Fly.io runs Zetta CRM well as a Docker app. Use an external MySQL 8 provider (PlanetScale, Aiven, RDS, DigitalOcean, etc.) or a Fly-hosted MySQL setup if you manage it yourself.

## 1. Launch app

```bash
fly launch --image ghcr.io/incredible-zetta/crm:v0.0.1-beta --no-deploy
```

Set internal port in `fly.toml`:

```toml
[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = false
  auto_start_machines = true
  min_machines_running = 1
```

## 2. Secrets

```bash
fly secrets set \
  MCP_API_KEY='<long random secret>' \
  BASE_URL='https://your-app.fly.dev' \
  DB_DSN='user:password@tcp(mysql-host:3306)/crmagents?parseTime=true&multiStatements=true' \
  SMTP_HOST='smtp.example.com' \
  SMTP_PORT='587' \
  SMTP_USER='no-reply@example.com' \
  SMTP_PASS='<password>' \
  SMTP_FROM='no-reply@example.com'
```

## 3. Volume for exports

```bash
fly volumes create zetta_exports --size 1
```

Mount in `fly.toml`:

```toml
[mounts]
  source = "zetta_exports"
  destination = "/data/exports"
```

## 4. Deploy

```bash
fly deploy --image ghcr.io/incredible-zetta/crm:v0.0.1-beta
fly status
curl -fsS https://your-app.fly.dev/healthz
```
