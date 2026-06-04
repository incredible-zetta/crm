# AWS ECS / Fargate install

Use ECS Fargate + ALB + RDS MySQL.

## Image

```text
ghcr.io/incredible-zetta/crm:v0.0.1-beta
```

Architecture tags are multi-platform (`linux/amd64`, `linux/arm64`).

## RDS MySQL

Create an RDS MySQL 8 instance. Put ECS task and RDS in the same VPC. Security group: ECS tasks may connect to RDS TCP 3306.

DSN:

```text
DB_DSN=user:password@tcp(rds-endpoint:3306)/crmagents?parseTime=true&multiStatements=true
```

Store secrets in AWS Secrets Manager or SSM Parameter Store.

## Task definition

Container port:

```text
8080
```

Environment:

```text
MCP_API_KEY=<secret>
BASE_URL=https://crm.example.com
DB_DSN=<secret>
EXPORT_DIR=/data/exports
```

For exports, use EFS mounted to:

```text
/data/exports
```

## Load balancer

- Listener: HTTPS 443
- Target group: HTTP 8080
- Health path: `/healthz`

## Notes

- Do not put `MCP_API_KEY` in plain task JSON if you commit it. Use secret references.
- Make `BASE_URL` exactly match the public ALB/CloudFront/custom-domain URL.
