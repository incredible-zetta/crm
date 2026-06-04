# Zetta CRM Wiki

Zetta CRM is a self-hosted MCP CRM for AI operators: contacts, templates, campaigns, email sending, tracking, scheduler, analytics, exports, and unsubscribe compliance.

## Install guides

- [Installation](Installation) — common Docker, Docker Compose, env vars, reverse proxy
- [EasyPanel](EasyPanel)
- [Coolify](Coolify)
- [Railway](Railway)
- [Render](Render)
- [Fly.io](Fly.io)
- [AWS ECS](AWS-ECS)
- [Google Cloud Run](Google-Cloud-Run)
- [Kubernetes](Kubernetes)

## Image

Release image:

```text
ghcr.io/incredible-zetta/crm:v0.0.1-beta
```

## MCP endpoint

```text
https://your-domain.example/mcp
```

Auth:

```text
Authorization: Bearer <MCP_API_KEY>
```

## Health check

```text
GET /healthz
```

Expected: HTTP 200.
