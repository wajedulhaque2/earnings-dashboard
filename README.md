# Earnings Dashboard

A Go/PostgreSQL application for personal earnings research with **£0 paid API subscriptions**. It uses direct Go clients for Yahoo Finance and SEC EDGAR. Missing financial observations stay SQL NULL and display as N/A. No Python, Node backend, paid provider dependency, or synthetic application seed data is required.

## Features

- Weekly calendar with BMO/AMC groups, explicit unknown timing, previous/current/next week navigation, company, sector, USD market-cap and watchlist filters.
- Arbitrary-symbol/company search: PostgreSQL first, optional explicit free-provider lookup, and queued ingestion.
- Company analysis with description, exchange, sector, industry, quote timestamp, market cap and next earnings.
- Quarterly revenue and diluted EPS charts, accessible source tables, historical surprise/reaction heatmap, and toggleable reaction chart.
- Premarket, event-day and 1/2/5/10/21/63 trading-session returns; per-horizon sample counts, average, median, win rate, best, worst and sample standard deviation.
- Four EPS/revenue beat/miss groups and five premarket-gap groups, excluding unavailable observations.
- Persistent default watchlist, background refresh queue, scheduled ingestion, provider status and graceful empty/error states.
- Embedded HTML/CSS and local ECharts assets; no frontend generation/build tool is needed.

Availability depends on the source. Historical revenue estimates/surprises, old premarket prices, logos and forward quarterly estimates are commonly unavailable. There are no invented substitutes. See [methodology](docs/METHODOLOGY.md) and [implementation/verification status](docs/IMPLEMENTATION_STATUS.md).

## Run with Docker

Prerequisite: Docker with Compose v2. From the repository root:

```sh
docker compose up --build -d
docker compose ps
docker compose logs migrate app worker
```

Open [the calendar](http://localhost:8080/calendar). Compose starts PostgreSQL 17, runs migrations, starts the web server and starts the scheduler. The first calendar refresh happens automatically; company enrichment is gradual. No provider key is required. Ports bind only to localhost. Database credentials in Compose are development defaults.

```sh
docker compose run --rm worker /app/worker sync-calendar
docker compose run --rm worker /app/worker sync-all NVDA
docker compose run --rm migrate /app/migrate status
docker compose down
```

Data persists in the `postgres_data` volume. `docker compose down -v` deletes it. After adding migrations, run `docker compose run --rm migrate /app/migrate up` before restarting app/worker. Avoid simultaneous manual and scheduled bulk ingestion; `docker compose stop worker` pauses the scheduler.

To enable SEC, copy `.env.example` to `.env` and set `SEC_USER_AGENT` to your application name plus real contact email. Blank leaves SEC unavailable and Yahoo supplies available fundamentals. SEC does not require an API key. Do not commit personal configuration.

## Run locally

Prerequisites: Go 1.24+ and PostgreSQL 17. Make is optional. Defaults match the Compose database:

```sh
docker compose up -d db
go mod download
go run ./cmd/migrate up
go run ./cmd/server
```

In another terminal:

```sh
go run ./cmd/worker sync-calendar
go run ./cmd/worker sync-all NVDA
go run ./cmd/worker schedule
```

For an existing PostgreSQL installation, create `earnings_dashboard` and set `DATABASE_URL` to a migration-capable account. Go reads process environment variables; it does not load `.env` automatically. PowerShell example:

```powershell
$env:DATABASE_URL = 'postgres://postgres:postgres@localhost:5432/earnings_dashboard?sslmode=disable'
$env:PORT = '8080'
go run ./cmd/migrate up
go run ./cmd/server
```

POSIX overrides use `export DATABASE_URL='...'`. Set `SEC_USER_AGENT` in the environment of both server and worker if enabling SEC.

| Variable | Default | Meaning |
| --- | --- | --- |
| APP_ENV | development | development, test or production |
| PORT | 8080 | HTTP port, 1–65535 |
| DATABASE_URL | postgres://postgres:postgres@localhost:5432/earnings_dashboard?sslmode=disable | Required explicitly in production |
| DATABASE_TIMEOUT | 5s | Database/health deadline; positive, at most 1m |
| SHUTDOWN_TIMEOUT | 10s | Graceful shutdown deadline; positive, at most 1m |
| SEC_USER_AGENT | empty | Descriptive application/contact identity; real email required for SEC |

Compose's `PORT` changes its host port only. U.S. market timezone is always America/New_York, with embedded timezone data.

## Ingestion commands

```sh
go run ./cmd/worker check
go run ./cmd/worker sync-company NVDA
go run ./cmd/worker sync-financials NVDA
go run ./cmd/worker sync-earnings NVDA
go run ./cmd/worker sync-prices NVDA
go run ./cmd/worker sync-intraday NVDA
go run ./cmd/worker sync-calendar
go run ./cmd/worker sync-all NVDA
go run ./cmd/worker recalculate NVDA
go run ./cmd/worker schedule
```

`sync-calendar` covers seven days back through fourteen days ahead. `sync-all`, earnings, prices and intraday commands recalculate available reactions. Repeated ingestion upserts the same records and preserves existing non-null provider fields. Derived reactions are recalculated rather than preserving obsolete calculations. Commands return a nonzero exit status for incomplete required operations; optional SEC failures are visible in `/status` while Yahoo fallback can still succeed.

Example development symbols are not a fixed universe. To load all examples sequentially in PowerShell:

```powershell
'NVDA','META','MU','LEN','AAPL','MSFT','AMZN','GOOGL','TSLA','AMD' | ForEach-Object {
    go run ./cmd/worker sync-all $_
    if ($LASTEXITCODE -ne 0) { Write-Warning "Incomplete refresh: $_" }
}
```

POSIX equivalent:

```sh
for symbol in NVDA META MU LEN AAPL MSFT AMZN GOOGL TSLA AMD; do
  go run ./cmd/worker sync-all "$symbol" || echo "Incomplete refresh: $symbol"
done
```

## Scheduling and operations

Run one `schedule` process. It holds a PostgreSQL advisory lock, works sequentially, and pauses one minute between bounded cycles. Calendar attempts are six hours apart. Company/result/price/fundamental refresh has a 24-hour cooldown, processes up to three due symbols per cycle and prioritizes watched companies. Manual refresh requests are persisted and retried at most five times with at least fifteen minutes between failures. Recent event intraday capture has an hourly cooldown, up to twenty symbols per cycle, on trading days after 09:30 through 20:59 New York time. This stores observations for future historical research; it is not a streaming quote service.

Use `/status` for provider/resource success and attempt times and `/health` for database/schema readiness. Health returns 200 when ready and 503 otherwise, without connection details. The web server can start during a database outage. Provider failures do not remove previously stored observations. Logs are structured JSON with safe error categories; credentials, cookies and response bodies are excluded.

## Checks

```sh
gofmt -w .
go vet ./...
go test ./...
go build ./...
```

Default tests use httptest fixtures and require neither a database nor live providers. PostgreSQL integration tests create/drop an isolated schema in an explicitly supplied disposable database:

```powershell
$env:TEST_DATABASE_URL = 'postgres://postgres:postgres@localhost:5432/earnings_dashboard?sslmode=disable'
go test -tags integration ./internal/repository -v
```

Do not point integration tests at production. They exercise migrations up/repeat/down/up, real SQL, null preservation, source precedence, watchlists, prices, reactions and refresh queues.

Make targets: `dev`, `worker`, `schedule`, `migrate`, `migrate-status`, `migrate-down`, `fmt`, `vet`/`lint`, `test`, `integration`, `build`, `check`. Migration down commands remove later schema/data; back up first.

## Structure and migrations

- `cmd/server`, `cmd/worker`, `cmd/migrate`: web, ingestion/scheduling and embedded Goose migration executables.
- `internal/config`, `database`, `models`, `repository`: configuration, pgx and normalized nullable records/SQL.
- `internal/providers/{yahoo,sec,httpclient}` and `providers.go`: isolated adapters, interfaces, deadlines, rate limiting and bounded retries.
- `internal/service`, `scheduler`: ingestion, reaction persistence and refresh orchestration.
- `internal/earnings`, `analytics`: trading sessions, formulas and statistical definitions.
- `internal/web`, `web/templates`, `web/static`: chi, html/template, plain responsive CSS and vendored ECharts with its license.
- `00001_initial.sql`: preserved Phase 1 schema.
- `00002_research.sql`: historical prices, snapshots, filings, refresh requests, period-based fundamentals, company metadata and reaction dates.
- `00003_provenance.sql`: per-field financial source and split-event provenance.
- `00004_refresh.sql`: persistent attempt timestamps and due-request index.

No optional Massive or Alpha Vantage adapter is implemented; no paid fallback is invoked.

## Deployment readiness

The Docker packaging, migrations, non-root image, graceful shutdown, health checks and persistent scheduler support a private single-user installation. Docker execution was not verified in the implementation environment because Docker was unavailable. Go binaries, actual PostgreSQL migrations/queries and Yahoo ingestion were tested locally; SEC was tested with fixtures only.

V1 has no authentication and should remain private. Before internet-facing deployment, add authentication at a trusted reverse proxy, HTTPS, appropriate database credentials/TLS, backups and restore testing, monitoring and resource limits. Do not expose the development database or default credentials. Yahoo is unofficial and may change or deny access. Review exchange holiday rules annually and account for exceptional closures. This is a research application, not an execution system or point-in-time backtesting dataset.
