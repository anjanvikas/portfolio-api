# portfolio-api

Go backend for [anjanvikas.dev](https://anjanvikas.dev). Serves projects, blog posts, and admin endpoints.

## Prerequisites

- Go 1.23+
- Docker & Docker Compose (for local Postgres + Redis)

## Local services (Postgres + Redis)

`docker-compose.yml` brings up Postgres 16 and Redis 7 with persistent named
volumes (`postgres_data`, `redis_data`). Data survives `docker compose down`.

| Service | Host port | In-container port | Volume |
|---------|-----------|-------------------|--------|
| Postgres 16 | `5433` | `5432` | `postgres_data` |
| Redis 7 | `6379` | `6379` | `redis_data` |

> Postgres maps to host port **5433** (not 5432) to avoid clashing with other
> local Postgres instances. The `.env.example` `DATABASE_URL` matches.

```bash
docker compose up -d        # start Postgres + Redis in the background
docker compose ps           # check health
docker compose logs -f      # tail logs
docker compose down         # stop containers (volumes are preserved)
docker compose down -v      # nuke containers AND data volumes (destructive)
```

Redis is provisioned for future use (caching, rate limiting in F16). No Go
handler reads from it yet.

## Quick start

```bash
# 1. Copy env file and fill in values
cp .env.example .env

# 2. Start local Postgres + Redis
docker compose up -d

# 3. Run the API (hot-reload via Air)
make dev
```

The server listens on `http://localhost:8080` by default.  
Health check: `GET /health` → `{"status":"ok"}`

If any required env var (see table below) is missing, the API logs the full
list of missing keys and exits with status 1 — it never starts in a half-configured state.

## Directory structure

```
cmd/api/          # main entry point
internal/
  handler/        # HTTP handlers
  service/        # business logic
  store/          # database queries
  middleware/     # auth, logging, CORS
pkg/              # shared, reusable utilities
```

## Available commands

```bash
make tools                       # install Air, golang-migrate, golangci-lint
make dev                         # start with Air hot-reload
make build                       # compile binary to ./bin/api
make run                         # build then run once (no hot reload)
make migrate                     # apply all pending migrations
make migrate-down                # roll back the last migration
make migrate-new name=add_users  # scaffold a new up/down migration pair
make seed                        # seed local DB (stubbed until entity stories)
make lint                        # golangci-lint
make test                        # go test ./...
make clean                       # remove bin/ and tmp/
make help                        # list all targets
```

Migrations live in `migrations/` as numbered `*.up.sql` / `*.down.sql` pairs
applied by [golang-migrate](https://github.com/golang-migrate/migrate). The
schema is the source of truth for sqlc codegen.

After cloning, run `make tools` once to install Air (`go install ...@latest`
puts binaries in `$GOPATH/bin` — make sure that's on your `PATH`).

## Environment variables

Copy `.env.example` to `.env` and fill in real values before running:

| Variable | Required | Description |
|----------|----------|-------------|
| `PORT` | no (default `8080`) | HTTP port |
| `DATABASE_URL` | yes | Postgres connection string |
| `JWT_SECRET` | yes | Secret for signing JWTs |
| `ADMIN_PASSWORD` | yes | Bcrypt-hashed admin password |
| `R2_ACCESS_KEY` | yes | Cloudflare R2 access key |
| `R2_SECRET_KEY` | yes | Cloudflare R2 secret key |
| `R2_BUCKET_NAME` | yes | R2 bucket name |
| `R2_ENDPOINT` | yes | R2 S3-compatible endpoint URL |
