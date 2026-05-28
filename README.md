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
Health check: `GET /api/v1/health` → `{"status":"ok","version":"...","db":"connected"}`

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
make tools                       # install Air, golang-migrate, sqlc, golangci-lint
make dev                         # start with Air hot-reload
make build                       # compile binary to ./bin/api
make run                         # build then run once (no hot reload)
make migrate                     # apply all pending migrations
make migrate-down                # roll back the last migration
make migrate-new name=add_users  # scaffold a new up/down migration pair
make sqlc                        # regenerate typed Go from queries/ + migrations/
make seed                        # seed local DB with sample data (idempotent)
make hashpw                      # prompt for a passphrase; print a bcrypt hash for ADMIN_PASSWORD
make lint                        # golangci-lint
make test                        # go test ./...
make clean                       # remove bin/ and tmp/
make help                        # list all targets
```

Migrations live in `migrations/` as numbered `*.up.sql` / `*.down.sql` pairs
applied by [golang-migrate](https://github.com/golang-migrate/migrate). The
schema is the source of truth for sqlc codegen. See
[**docs/migrations.md**](../docs/migrations.md) for the full workflow,
command reference, and Neon branch strategy.

After cloning, run `make tools` once to install Air (`go install ...@latest`
puts binaries in `$GOPATH/bin` — make sure that's on your `PATH`).

## Environment variables

Copy `.env.example` to `.env` and fill in real values before running:

| Variable | Required | Description |
|----------|----------|-------------|
| `PORT` | no (default `8080`) | HTTP port |
| `DATABASE_URL` | yes | Postgres connection string |
| `JWT_SECRET` | yes | HS256 signing key for admin JWTs (32+ random bytes) |
| `ADMIN_PASSWORD` | yes | **Bcrypt hash** of the admin passphrase — generate with `make hashpw` |
| `CORS_ALLOWED_ORIGINS` | no (default `http://localhost:3000`) | CSV of origins allowed by CORS |
| `COOKIE_SECURE` | no (default `false`) | Set `true` behind HTTPS so logout's `Set-Cookie` carries `Secure` |
| `R2_ACCESS_KEY` | yes | Cloudflare R2 access key |
| `R2_SECRET_KEY` | yes | Cloudflare R2 secret key |
| `R2_BUCKET_NAME` | yes | R2 bucket name |
| `R2_ENDPOINT` | yes | R2 S3-compatible endpoint URL |

`.env` is loaded at startup by the binaries themselves (`config.LoadDotEnv`) rather than `make include`, so values that contain `$` (notably the bcrypt hash) pass through unmangled.

## Auth surface

Single-admin model. There is no user table; the bcrypt hash in `ADMIN_PASSWORD` is the only credential.

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| `POST` | `/api/v1/admin/login` | none | Body `{"password": "..."}`. Returns `{"token": "..."}`. Rate-limited to 5 failed attempts per IP per 15 minutes (in-memory). |
| `POST` | `/api/v1/admin/logout` | none | Clears the `admin_token` cookie on the API domain. Unauthenticated so an expired session can still log out. |
| `*` | `/api/v1/admin/...` (future routes) | Bearer JWT | `RequireAdmin` middleware validates HS256, exp, and `role: "admin"`. Admin subject lands on the request context as `mw.AdminIDFromContext(ctx)`. |

Generate the hash interactively:

```bash
make hashpw
# Passphrase: ********
# $2a$10$...
```

…then paste the printed line into `.env` as `ADMIN_PASSWORD=$2a$10$...`.
