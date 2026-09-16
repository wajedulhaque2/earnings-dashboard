# Codex Start Here

When opening this repository in Codex, begin with this task:

```text
Read AGENTS.md and docs/PRODUCT_SPEC.md in full before making changes.

Inspect the repository and implement Phase 1 only.

Phase 1 requirements:
- initialize the Go module and production-quality repository structure;
- add configuration loading with sensible development defaults;
- add PostgreSQL connectivity using pgx;
- create initial SQL migrations for companies, earnings_events, quarterly_financials, earnings_reactions, watchlists, watchlist_companies, and provider_sync_state;
- add repository/database scaffolding;
- create a chi HTTP server;
- add GET /health that reports application/database health;
- create a minimal base web layout suitable for the later calendar UI;
- add Dockerfile and docker-compose.yml for app + PostgreSQL;
- add a Makefile with dev, test, build, lint/vet, migrate and worker targets where appropriate;
- add meaningful tests for the configuration/database-independent pieces you introduce;
- update README.md with exact local setup and run instructions.

Constraints:
- API/data spend must remain £0;
- do not add any paid provider dependency;
- do not add fake market/earnings data;
- do not start implementing provider ingestion yet except interfaces/scaffolding if Phase 1 needs them;
- keep the design simple and Go-first;
- avoid a React SPA.

Before finishing:
1. run gofmt -w .
2. run go vet ./...
3. run go test ./...
4. run go build ./...
5. fix every failure

Do not merely describe the files. Create and modify the repository files yourself.

When Phase 1 is complete, summarize exactly what works, any assumptions made, and the next Phase 2 tasks. Do not begin Phase 2 until Phase 1 is healthy.
```

## Phase 2 prompt

After Phase 1 is working:

```text
Read AGENTS.md and docs/PRODUCT_SPEC.md again. Inspect the current implementation before changing anything.

Implement Phase 2: the free-data company/search/earnings ingestion foundation.

Priorities:
1. Yahoo provider abstraction and implementation for ticker/company lookup, company profile data, earnings dates, historical EPS estimate/actual/surprise where available, and long-history daily prices.
2. SEC EDGAR client foundation for official U.S. company metadata/filings/financial statement facts.
3. Earnings calendar ingestion using free sources only.
4. Search that supports arbitrary tickers/company names rather than a hardcoded universe.
5. Persist normalized provider data into PostgreSQL.
6. Provider rate limiting, timeouts, structured errors, and graceful NULL handling.

Do not depend on a paid API. Massive Free / Alpha Vantage Free may be added only as optional fallback adapters and only if useful within their free limits.

Use these symbols for development validation only: NVDA, META, MU, LEN, AAPL, MSFT, AMZN, GOOGL, TSLA, AMD. Do not hardcode them as the universe.

Never fabricate missing values. If a field cannot be sourced reliably for free, store NULL.

Run gofmt, go vet ./..., go test ./..., and go build ./... before finishing and fix failures.
```
