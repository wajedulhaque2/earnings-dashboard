# Earnings Dashboard

A personal earnings research application with **£0 API/data spend**. Missing
financial values remain NULL and display as N/A. Phase 1 contains no provider
calls or fabricated financial data.

## What works

Go configuration, pgx PostgreSQL pool, seven-table Goose migration, repository
readiness checks, chi server, embedded HTML/CSS calendar shell, JSON health,
graceful shutdown, and a one-shot worker readiness command. `/` redirects to
`/calendar`. The calendar displays an explicit empty state. Search, ingestion,
weekly navigation, stock analysis and watchlist controls belong to later phases.

## Docker setup

Install Docker with Compose v2. From the repository root:

```sh
docker compose up --build -d
docker compose ps
docker compose logs migrate app
```

Open http://localhost:8080/calendar and http://localhost:8080/health. Compose
starts PostgreSQL 17, waits for readiness, runs migrations, then starts the app.
Ports bind to localhost only. Fixed database credentials are for development.
No provider keys are needed.

```sh
docker compose run --rm worker
docker compose run --rm migrate /app/migrate status
docker compose down
```

Database data persists in `postgres_data`. `docker compose down -v` **deletes the
database**. After adding migrations, run `docker compose run --rm migrate`
before restarting the app.

## Local Go setup

Prerequisites: Go 1.24+ and PostgreSQL 17, or Docker for PostgreSQL. Make is
optional; the direct Go commands also work in PowerShell.

```sh
docker compose up -d db
go mod download
go run ./cmd/migrate up
go run ./cmd/server
```

For an existing PostgreSQL server, create the `earnings_dashboard` database and
set `DATABASE_URL` to an account with migration permissions. Go reads process
environment variables; it does **not** automatically load `.env`. Defaults work
with the Compose database without a `.env` file.

PowerShell overrides:

```powershell
$env:PORT = '9090'
$env:DATABASE_TIMEOUT = '3s'
go run ./cmd/server
```

POSIX shell equivalent:

```sh
PORT=9090 DATABASE_TIMEOUT=3s go run ./cmd/server
```

| Variable | Development default | Validation |
| --- | --- | --- |
| APP_ENV | development | development, test, production |
| PORT | 8080 | 1–65535 |
| DATABASE_URL | postgres://postgres:postgres@localhost:5432/earnings_dashboard?sslmode=disable | Explicitly required in production |
| DATABASE_TIMEOUT | 5s | Positive duration up to 1m |
| SHUTDOWN_TIMEOUT | 10s | Positive duration up to 1m |

Compose uses its internal database URL. Its optional `PORT` environment/.env
value changes the host port only. Provider variables in `.env.example` are
reserved for later phases and are not read yet. Never commit real secrets.

## Health and migrations

`GET /health` returns 200 with `{"application":"ok","database":"ok"}` when
PostgreSQL is reachable and all seven tables exist. Otherwise it returns 503
with database `unavailable`. Checks have a deadline and expose no connection
details. The server starts during database outages; pgx reconnects when available.

```sh
go run ./cmd/migrate up
go run ./cmd/migrate status
go run ./cmd/worker
```

`go run ./cmd/migrate down` rolls back the latest migration and **destroys its
tables and data**. Goose tracks versions so repeated up commands are safe.
The worker checks database/schema readiness and exits; no jobs are scheduled.

Make targets: `dev`, `test`, `build`, `fmt`, `lint`/`vet`, `migrate`,
`migrate-down`, `migrate-status`, `worker`, `check`. Build writes to `bin/`.

## Required checks

```sh
gofmt -w .
go vet ./...
go test ./...
go build ./...
```

Unit tests need no database. They cover configuration defaults, overrides,
validation and DST, pool settings and credential-safe errors, health status
and deadlines, routes and embedded assets.

## Structure and assumptions

- `cmd/server`, `cmd/migrate`, `cmd/worker`: executable entry points.
- `internal/config`, `internal/database`, `internal/repository`, `internal/web`:
  application foundation; repositories accept a pgx pool or transaction.
- `migrations`: embedded reversible SQL, independent of working directory.
- `web/templates`, `web/static`: embedded standard-library templates and CSS.
  No frontend build; templ, HTMX and charting can be introduced when needed.

All seven specified tables exist. Financial fields are nullable with no zero
substitution. Only an empty Default watchlist is seeded. Uniqueness applies to
company/report date, company/fiscal quarter, reaction/event, and provider/resource.
Phase 2 ingestion must reconcile changed report dates. Timestamps use TIMESTAMPTZ,
connections use UTC, and US market dates use America/New_York with embedded
zone data. Repositories must explicitly maintain `updated_at`. No return formulas
or trading-calendar calculations exist yet; future analytics must define units
in their methodology version.

## Next: Phase 2

Add free Yahoo provider interfaces/adapters for arbitrary-symbol search, profiles,
earnings and daily prices; SEC client foundations; normalized persistence with
parameterized upserts, rate limits, bounded retries, timeouts and safe errors.
Normal pages must read PostgreSQL rather than make provider requests.

See [AGENTS.md](AGENTS.md), [product specification](docs/PRODUCT_SPEC.md), and
[phase instructions](docs/CODEX_START_HERE.md).
