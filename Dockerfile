# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.25 AS build
WORKDIR /src

# Cache deps
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /crm-server ./cmd/server

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot

# Export dir for generated CSVs (mount a volume here on EasyPanel for persistence)
WORKDIR /data
ENV EXPORT_DIR=/data/exports

COPY --from=build /crm-server /crm-server

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/crm-server"]
