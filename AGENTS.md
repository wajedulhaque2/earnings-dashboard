# Codex Instructions — Earnings Dashboard

This repository is for a personal earnings-trading research application.

## Non-negotiable rules

1. API/data cost must remain **£0**. Do not add any dependency that requires a paid API plan for core functionality.
2. Allowed sources for V1:
   - Yahoo Finance / yfinance
   - SEC EDGAR
   - Massive Free
   - Alpha Vantage Free
3. If a field cannot be obtained reliably for free, store `NULL` and display `N/A`.
4. Never fabricate, infer, or silently substitute financial data.
5. Keep every external data source behind a provider interface.
6. Pages should normally read from PostgreSQL, not call third-party providers directly on every page load.
7. Go is the main application language.
8. PostgreSQL is the database.
9. Use `America/New_York` when interpreting U.S. earnings sessions and market times.
10. Use trading sessions, not calendar days, for forward-return calculations.
11. Never expose provider keys or secrets in frontend code, logs, commits, fixtures, or screenshots.
12. Prefer simple, readable Go over unnecessary abstraction or microservices.
13. Do not leave core functionality as placeholder TODOs unless blocked by credentials or an unavailable free-data field.
14. Do not hardcode a small ticker universe. The finished product must support arbitrary symbols available from the configured free sources.

## Product focus

This is an earnings research tool, not a general-purpose investing portal.

The primary workflows are:

- browse a weekly earnings calendar;
- separate Before Market Open (BMO) and After Market Close (AMC) reports;
- click a company to open a detailed earnings-analysis page;
- inspect quarterly revenue and diluted EPS;
- inspect historical EPS/revenue surprises where free data is available;
- inspect premarket/event-day/forward price reactions;
- inspect summary statistics and conditional earnings behaviour;
- maintain a simple personal watchlist.

## Preferred stack

- Go 1.24+
- chi
- PostgreSQL
- pgx
- templ
- HTMX
- Tailwind CSS
- ECharts
- Docker / docker-compose
- Goose or golang-migrate

Avoid a React SPA unless there is a concrete technical reason that cannot be met with server-rendered Go + HTMX.

## Suggested repository layout

```text
cmd/
  server/
  worker/
internal/
  config/
  database/
  models/
  repository/
  service/
  providers/
    yahoo/
    sec/
    massive/
    alphavantage/
  earnings/
  marketdata/
  scheduler/
  web/
web/
  templates/
  static/
migrations/
docs/
```

## Provider priority

Use the following priorities unless the implementation proves a better free-data ordering:

### Company profile / description
1. Yahoo
2. SEC where applicable
3. `N/A`

### Daily prices
1. Yahoo
2. Massive Free
3. `N/A`

### Quarterly reported financials
1. SEC EDGAR for U.S. issuers
2. Yahoo
3. `N/A`

### Earnings dates / EPS estimate / reported EPS / EPS surprise
1. Yahoo
2. Alpha Vantage Free
3. `N/A`

### Earnings calendar
1. Yahoo
2. Alpha Vantage Free
3. `N/A`

### Premarket move
1. our own stored historical observation
2. recent Yahoo intraday/pre-post data
3. Massive Free where available
4. `N/A`

Never attempt to reconstruct old intraday/premarket prices from daily OHLC data.

## Earnings-session methodology

For U.S. equities, regular session is 09:30–16:00 America/New_York.

- Tuesday AMC release -> Wednesday is the event trading session.
- Wednesday BMO release -> Wednesday is the event trading session and Tuesday is previous close.
- Friday AMC -> next valid trading session, respecting U.S. market holidays.

Do not equate earnings report date with reaction trading date without considering BMO/AMC.

## Return methodology

Centralise all formulas in one analytics package.

- Premarket return = premarket price / previous close - 1
- Event-day return = event close / previous close - 1
- Forward returns begin from event-day close.
- 1D = 1 trading session after event close
- 2D = 2 sessions
- 1W = 5 sessions
- 2W = 10 sessions
- 1M = 21 sessions
- 3M = 63 sessions

NULL observations must be excluded from averages, medians, win rates, extrema, standard deviation, and sample counts.

## UI direction

The site should look like a purpose-built financial research product:

- light neutral background;
- dark text;
- restrained blue accents;
- subtle borders;
- compact information density;
- muted red/green heatmap cells;
- no excessive gradients, giant cards, or gimmicky animations.

The primary desktop target is approximately 1440px width. Tables may horizontally scroll on smaller screens.

## Development process

Implement in phases. Do not generate a huge unverified code dump.

### Phase 1
- module/project structure
- config
- PostgreSQL connection
- migrations
- repository layer
- health endpoint
- base web layout
- Docker development environment
- Makefile
- tests

### Phase 2
- Yahoo provider
- company lookup/profile ingestion
- earnings calendar ingestion
- search

### Phase 3
- weekly earnings calendar UI
- BMO/AMC split
- logos where available
- stock/company pages

### Phase 4
- quarterly financials
- EPS/revenue charts
- earnings history and surprise data

### Phase 5
- market data provider(s)
- reaction engine
- trading-session handling
- forward returns

### Phase 6
- heatmap table
- summary statistics
- conditional analytics

### Phase 7
- watchlist
- scheduled workers
- resilience/error states
- UI refinement

### Phase 8
- tests
- documentation
- deployment readiness

## Required checks before completing a task

Run and fix failures from:

```bash
gofmt -w .
go vet ./...
go test ./...
go build ./...
```

If frontend generation/build tools are introduced, run their required checks too.

## Security and reliability

- Validate symbol/path inputs.
- Use parameterized SQL.
- Use HTTP timeouts.
- Retry transient provider failures with bounded exponential backoff.
- Rate-limit provider requests to remain inside free limits.
- Use structured logging.
- Do not log credentials.
- Make ingestion jobs idempotent.
- Missing optional data must not crash pages.

## Seed symbols for development

Use these only as test/seed examples, never as a hardcoded universe:

`NVDA`, `META`, `MU`, `LEN`, `AAPL`, `MSFT`, `AMZN`, `GOOGL`, `TSLA`, `AMD`.

## Before making architectural changes

Read this file and `docs/PRODUCT_SPEC.md` first. Preserve the zero-cost data constraint unless the user explicitly changes it.
