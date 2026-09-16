# Earnings Dashboard

A personal earnings-trading research dashboard focused on historical earnings reactions, quarterly fundamentals, and a weekly earnings calendar.

## Core principles

- Zero paid API spend.
- Use free data sources only.
- Never fabricate missing financial data; show `N/A` instead.
- Prefer Yahoo Finance/yfinance for long-history market data and company/earnings metadata.
- Prefer SEC EDGAR for official U.S. filings and financial statement validation.
- Massive Free and Alpha Vantage Free may be used only within their free-tier limits.
- Keep external data providers behind interfaces so they can be replaced later.
- Store fetched data in PostgreSQL and serve normal page requests from the database rather than repeatedly calling providers.

## Planned stack

- Go 1.24+
- chi router
- PostgreSQL + pgx
- templ
- HTMX
- Tailwind CSS
- ECharts
- Docker / docker-compose

## Product scope

The home page is a Monday-Friday earnings calendar with Before Open and After Close sections. Clicking a ticker opens a stock analysis page containing company information, quarterly revenue/EPS charts, historical EPS/revenue surprises, an earnings-reaction heatmap, forward-return statistics, and conditional analysis by earnings result and initial gap.

See `AGENTS.md` for the repository rules Codex must follow and `docs/PRODUCT_SPEC.md` for the detailed build specification.
