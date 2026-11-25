# portfolio-api — backend conventions

Go 1.26 · Chi · pgx/v5 · sqlc · golang-migrate · slog. Module `github.com/anjanvikas/portfolio-api`.
Structure and endpoints: `../docs/architecture/backend.md`, `../docs/architecture/api.md`.

## Layout
- `cmd/api` (server), `cmd/seed`, `cmd/hashpw`, `cmd/gen-homepage-og`
- `internal/handler`: HTTP handlers, DTOs, `router.go`. `internal/service`: logic that spans queries. `internal/store`: sqlc output plus the hand-written `pool.go`.
- `queries/*.sql`: one file per entity. `migrations/NNNNNN_<slug>.{up,down}.sql`.

## Rules
- Schema changes: `make migrate-new name=<slug>`, then write both up and down. Add the up-file to `sqlc.yaml` → `schema`. Never edit a committed migration.
- Queries: edit `queries/*.sql`, then `make sqlc`. Never edit generated `*.sql.go`, `db.go`, or `models.go`.
- Handlers take a small interface (`type projectQueries interface{…}`), not `*store.Queries`, so tests use fakes. Copy `handler/projects.go` and `projects_test.go` as the template.
- JSON: arrays are `[]`, never `null`. Dates are ISO `YYYY-MM-DD` via `isoDate`. UUIDs via `uuidString`. Errors are `{"error": "..."}`, and field validation is `{"errors": {field: msg}}` with status 400.
- Admin endpoints are mounted under the `RequireAdmin` subrouter in `router.go`.
- sqlc gotcha: aliasing a nullable expression (e.g. `NULLIF(...) AS x`) can be typed non-null and 500 on scan. Prefer NOT NULL columns or explicit `sqlc.narg`.
- `.env` is loaded by the binaries (`config.LoadDotEnv`), not by make. It may point at a shared Neon DB, so confirm before `migrate`, `seed`, or `migrate-down`.

## Done means
`go build ./... && go vet ./... && go test ./...` all pass. Edits are gofmt'd by a hook.
