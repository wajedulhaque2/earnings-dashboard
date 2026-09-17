# Implementation and verification status

## Delivered scope

Phase 1 was preserved and extended through provider/ingestion, calendar/company UI, reaction/analytics, watchlist/scheduling and final verification checkpoints. Backend, providers, workers, analytics, persistence and server rendering are Go. Browser JavaScript is limited to local ECharts rendering. Plain CSS and html/template avoid a frontend build dependency.

Working capabilities include weekly navigation and filters; BMO/AMC/unknown grouping; arbitrary-symbol/name search; persisted company pages and descriptions; nullable financial charts and source tables; EPS surprise; historical reaction heatmaps and chart; eight return horizons including premarket; all requested summary statistics; four beat/miss groups; five gap buckets; watchlist add/remove/filter; refresh queue; scheduled calendar, company, earnings-result, daily-price, fundamentals and recent-intraday updates; health and provider status; and graceful empty/provider/database error states.

## Provider capabilities and limits

| Provider | Implemented | Verification |
| --- | --- | --- |
| Yahoo Finance | Go HTTP search, profile/reference quote, earnings/calendar EPS fields, quarterly revenue/EPS, ten-year daily prices and split metadata, recent five-day pre/post minute bars | httptest fixtures and live NVDA ingestion/calendar |
| SEC EDGAR | Ticker/CIK lookup, company metadata, submission/filing metadata, company facts and direct quarterly revenue/diluted EPS | Fixtures only; live access requires a real descriptive contact User-Agent |
| Massive / Alpha Vantage | No adapter; no subscription or dependency | Not used |

Unavailable fields stay NULL/N/A. In particular historical revenue estimates/surprises, old premarket observations, company logos, explicit historical fiscal labels, future-quarter chart estimates and unsupported SEC fact tags may be absent. Source coverage can be shorter than the desired 8–12 quarters. In the live Yahoo NVDA check, five quarters were returned. Conditional/gap analytics are implemented and tested but display zero samples when their required source data is absent.

## Files and schema

See README's structure list for executable/package ownership. Main additions are provider interfaces and Yahoo/SEC/HTTP clients; normalized models; repository reads/upserts; ingestion/recalculation services; earnings calendar and analytics packages; scheduler; web pages/templates/CSS/chart assets and package tests. There are no Python or Node backend packages.

The initial migration remains intact. Additional migrations:

1. `00002_research.sql`: company metadata, nullable period-based fundamentals, event/reaction dates, historical prices, snapshots, filings and queued requests.
2. `00003_provenance.sql`: independent revenue/EPS sources and split ratios.
3. `00004_refresh.sql`: durable refresh attempt time and retry/due lookup index.

No market or earnings fixtures are seeded into the application database. Synthetic values exist only inside isolated tests. The only initial application seed is an empty default watchlist.

## Checks performed

- `gofmt -w .`, `go vet ./...`, `go test ./...`, `go build ./...`: passed at implementation checkpoints and final review.
- PostgreSQL integration test: passed against a disposable local PostgreSQL database. Verified migration up/repeat/down/up and real parameterized SQL for idempotent upserts, non-null preservation, financial source priority, event timing, watchlists, prices, snapshots, reactions, sync state and request queue.
- Unit coverage: configuration, symbol normalization, credential-safe errors, Yahoo/SEC parsing, missing/malformed data, 404/429/5xx/cancellation handling, BMO/AMC/Friday/holiday/DST mappings, all return horizons, current-session withholding, split/premarket incompatibility, NULL statistics, beat/miss and gap boundaries/counts/continuation, ingestion idempotency, scheduler deduplication/cancellation, HTTP pages/error states and origin checks.
- Live Yahoo: NVDA full ingestion produced 41 earnings events, 2,512 daily bars, five quarterly observations, 1,878 recent premarket snapshots and 40 event-day reactions in the temporary validation database. Upcoming calendar ingestion also succeeded. Counts are verification observations, not promised provider coverage.
- Browser: rendered populated calendar and NVDA charts; verified local company-name search and persisted watchlist add/remove. Chart pages had no browser warning/error logs. Responsive calendar tested at a narrow mobile viewport; columns stack vertically.

All default tests are offline. Live checks were separate explicit runtime validation. Test tools/data are outside the repository; the production database is not prepopulated by this work.

## Not verified locally

- Docker image build and Compose execution: Docker was unavailable.
- Live SEC endpoint access: no real contact User-Agent was supplied; no invented identity was used.
- Long-running scheduler soak, sustained multi-process failover, internet-facing deployment, backup restore and exhaustive symbol/exchange coverage.
- Runtime ingestion of every example symbol; the end-to-end live check used NVDA.

## Known operational limitations

Yahoo access is unofficial and may fail or change without notice. Limits are per provider client/process; multiple manually launched workers can multiply traffic. Scheduled operations are bounded and serialized, but this is a personal deployment, not a distributed work queue. The scheduler's advisory lock is held by a database connection; restart workers after database failures. A large universe is enriched gradually, with watchlist priority. Recent intraday availability is short-lived, so missing capture windows cannot be recovered from daily prices.

Changed scheduled earnings dates without stable provider identity can leave stale date rows; no speculative reconciliation is performed. The maintained U.S. calendar needs future exceptional-closure updates. Only explicitly identified New York market instruments get reaction calculations. Financials use latest reported/restated facts rather than point-in-time snapshots. Provider-field provenance is retained, but this is not a full audit archive of every raw response.

Deployment is suitable for a private single-user trial after verifying Docker on the target host and configuring the database/optional SEC identity. It is not ready for unauthenticated public internet exposure. Use authentication/TLS at a reverse proxy, non-default credentials, backups, monitoring and resource limits before wider deployment. No paid data service is required or started.
