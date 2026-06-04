# Kubernetes install

Deploy Zetta CRM with one Deployment, one Service, one Ingress, and MySQL 8 (managed or in-cluster).

## Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: zetta-crm-secrets
stringData:
  MCP_API_KEY: change-me-long-random
  DB_DSN: user:password@tcp(mysql:3306)/crmagents?parseTime=true&multiStatements=true
```

## Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zetta-crm
spec:
  replicas: 1
  selector:
    matchLabels:
      app: zetta-crm
  template:
    metadata:
      labels:
        app: zetta-crm
    spec:
      containers:
        - name: zetta-crm
          image: ghcr.io/incredible-zetta/crm:v0.0.1-beta
          ports:
            - containerPort: 8080
          env:
            - name: BASE_URL
              value: https://crm.example.com
            - name: EXPORT_DIR
              value: /data/exports
            - name: MCP_API_KEY
              valueFrom:
                secretKeyRef:
                  name: zetta-crm-secrets
                  key: MCP_API_KEY
            - name: DB_DSN
              valueFrom:
                secretKeyRef:
                  name: zetta-crm-secrets
                  key: DB_DSN
          volumeMounts:
            - name: exports
              mountPath: /data/exports
          readinessProbe:
            httpGet:
              path: /healthz
              port: 8080
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
      volumes:
        - name: exports
          persistentVolumeClaim:
            claimName: zetta-crm-exports
```

## Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: zetta-crm
spec:
  selector:
    app: zetta-crm
  ports:
    - port: 80
      targetPort: 8080
```

## PVC

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: zetta-crm-exports
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 1Gi
```

Add your preferred Ingress controller/cert-manager config with TLS and route host `crm.example.com` to service `zetta-crm:80`.
