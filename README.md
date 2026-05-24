# portfolio-api

Go backend for [anjanvikas.dev](https://anjanvikas.dev). Serves projects, blog posts, and admin endpoints.

## Prerequisites

- Go 1.23+
- Docker & Docker Compose (for local Postgres + Redis)

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
make dev       # start with Air hot-reload
make build     # compile binary to ./bin/api
make migrate   # run DB migrations
make seed      # seed local DB
make lint      # golangci-lint
make help      # list all targets
```

## Environment variables

Copy `.env.example` to `.env` and fill in real values before running:

| Variable | Description |
|----------|-------------|
| `PORT` | HTTP port (default `8080`) |
| `DATABASE_URL` | Postgres connection string |
| `JWT_SECRET` | Secret for signing JWTs |
| `ADMIN_PASSWORD` | Hashed password for admin login |
| `R2_ACCESS_KEY` | Cloudflare R2 access key |
| `R2_SECRET_KEY` | Cloudflare R2 secret key |
| `R2_BUCKET_NAME` | R2 bucket name |
| `R2_ENDPOINT` | R2 S3-compatible endpoint URL |
