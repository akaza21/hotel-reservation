# Hotel Reservation API

Backend service for hotel search, booking, and user management with Go + Fiber, MongoDB, and Redis.

## Features
- JWT authentication with role-aware access to admin endpoints.
- Hotel, room, and booking CRUD backed by MongoDB with validation and overlap checks.
- Redis-backed event bus and notification service plus rate limiting middleware.
- Structured logging, Prometheus metrics (`/metrics`), and Kubernetes manifests for API + monitoring stack.
- Comprehensive seed script and Postman/OpenAPI assets for quick demos.

## Prerequisites

- `source scripts/env.sh` (installs Go 1.24.4 into `.tools/`)
- Local MongoDB & Redis (use Docker or `scripts/run-mongo.sh` / `scripts/run-redis.sh`)
- `.env` based on `.env.example`

## Repository Layout

- `api/` – Fiber handlers, JWT middleware, error helpers.
- `db/` – MongoDB stores and fixtures.
- `internal/` – config, logging, middleware, caching, events, services.
- `scripts/` – bootstrap scripts (`env`, `run-mongo`, `run-redis`, `local-*`).
- `k8s/` – base manifests plus monitoring stack.
- `types/` – domain models (User, Hotel, Room, Booking).
- `docs/openapi.yaml` – REST contract.

## Run Locally

```bash
git clone <repo> && cd hotel-reservation
source scripts/env.sh
cp .env.example .env
./scripts/run-mongo.sh      # optional if Docker available
./scripts/run-redis.sh
go run main.go              # http://127.0.0.1:5000
```

Health: `GET /health`  
Metrics: `GET /metrics`

## Deploy Options

| Mode | Steps |
|------|-------|
| Bare metal | `scripts/env.sh`, run local Mongo/Redis, `go run main.go` |
| Docker Desktop / Minikube | `./scripts/local-deploy.sh` (or `.bat`) then `kubectl port-forward` (`hotel-reservation-api`, `prometheus-server`, `grafana`) |
| Other clusters | Apply `k8s/base` + overlays manually; wire ingress/TLS per environment |

Production tips: override secrets, point Mongo/Redis at managed services, and enable TLS/Ingress before exposing publicly.

### Kubernetes Secrets

Create the referenced secrets before deploying:

```bash
kubectl create namespace hotel-reservation-dev
kubectl create namespace monitoring

kubectl -n hotel-reservation-dev create secret generic api-secret \
  --from-literal=JWT_SECRET="$(openssl rand -hex 32)" \
  --from-literal=MONGO_DB_URL="mongodb://admin:<password>@mongodb-service:27017"

kubectl -n hotel-reservation-dev create secret generic notification-secret \
  --from-literal=SMTP_USERNAME="your-email@example.com" \
  --from-literal=SMTP_PASSWORD="your-smtp-password" \
  --from-literal=TWILIO_ACCOUNT_SID="ACxxxxxxxx" \
  --from-literal=TWILIO_AUTH_TOKEN="xxxxxxxx"

kubectl -n monitoring create secret generic grafana-secret \
  --from-literal=admin-user="admin" \
  --from-literal=admin-password="strong-password"
```

Provide real values per environment, then run `./scripts/local-deploy.sh` (or apply manifests manually).

## Configuration (env)

Key vars: `PORT`, `MONGO_DB_URL`, `MONGO_DB_NAME`, `JWT_SECRET`, `LOG_LEVEL`, plus rate limiter knobs (`RATE_LIMIT_ENABLED`, `RATE_LIMIT_REQUESTS`, `RATE_LIMIT_WINDOW`, `RATE_LIMIT_CRITICAL_*`).

## Observability

- OpenAPI spec: `docs/openapi.yaml`
- `/metrics` exposes Prometheus data; deploy `k8s/monitoring/` for Prometheus + Grafana.
- JSON logs (logrus) suitable for ELK/Loki pipelines.

## Testing

```bash
source scripts/env.sh
GOPATH=$PWD/.gopath GOMODCACHE=$PWD/.gopath/pkg/mod go test ./...
```

Set `RUN_DB_TESTS=1` and run Mongo locally to include the integration test.
