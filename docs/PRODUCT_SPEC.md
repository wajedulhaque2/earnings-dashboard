# September 2026 strategy amendment

The current production strategy supersedes earlier broad-universe and premarket requirements below. Track approximately 750–1000 attribute-qualified US-listed operating equities; persist eligibility and exclusion reasons. The default calendar and background ingestion are restricted to eligible companies, with current/next earnings first. Arbitrary-symbol search remains available and identifies outside-universe results. Opening Gap from daily OHLC replaces historical premarket as the core metric. EPS-only beat/miss and large-surprise groups replace the primary EPS/revenue matrix. Preserve optional intraday and revenue data, hide empty analytical grids, and maintain a repeatable audit of at least 50 representative companies. See [exact policy](UNIVERSE.md).

---

# Earnings Dashboard — Product Specification

## Goal

Build a personal research website for trading around quarterly earnings reports. The core experience is a weekly earnings calendar and a detailed per-stock page showing how the company has historically behaved around earnings.

The product must operate with **zero paid API subscriptions**.

## Core pages

### 1. Weekly earnings calendar

Route: `/calendar`

Display Monday through Friday. Each day is split into:

- Before Open
- After Close

Each company item should show, where available:

- company logo
- company name
- ticker
- optional market cap

Filters:

- market cap: All, >$500M, >$1B, >$5B, >$10B, >$50B
- session: All, Before Open, After Close
- sector
- Watchlist only
- ticker/company search

Navigation:

- Previous Week
- This Week
- Next Week

### 2. Stock analysis page

Route: `/stocks/{symbol}`

Header:

- company logo
- company name
- ticker
- latest known price
- market cap
- sector
- industry
- exchange
- next earnings date
- BMO/AMC badge
- company description

#### Quarterly fundamentals

Two side-by-side charts:

1. Quarterly Revenue
2. Quarterly Diluted EPS

Show approximately 8–12 historical quarters and, where a reliable free estimate is available, the next-quarter estimate as an outlined/dashed value.

#### Historical earnings-reaction table

Columns:

- Report Date
- Quarter
- Revenue Surprise %
- EPS Surprise %
- Premarket Move
- Event Day Move
- 1D Later
- 2D Later
- 1W Later
- 2W Later
- 1M Later
- 3M Later

Positive values use muted green cells. Negative values use muted red cells. Missing data is `N/A`.

Default sort: newest first.

#### Summary statistics

Columns:

- Premarket
- Event Day
- 1D
- 2D
- 1W
- 2W
- 1M
- 3M

Rows:

- Win %
- Median
- Average
- Best
- Worst
- Standard Deviation
- Sample Size

Only valid, non-NULL observations count.

#### Earnings-condition analytics

Group historical events into:

1. EPS Beat + Revenue Beat
2. EPS Beat + Revenue Miss
3. EPS Miss + Revenue Beat
4. EPS Miss + Revenue Miss

For each group show:

- Events
- Average Event Day
- Event Day Win %
- Average 1W
- Average 1M

#### Initial-gap analytics

Where a reliable premarket observation exists, bucket events into:

- Gap > +5%
- Gap +2% to +5%
- Gap -2% to +2%
- Gap -5% to -2%
- Gap < -5%

For each bucket show:

- Sample Size
- Event Day Continuation
- 1W Average
- 1M Average
- Win Rate

#### Historical reaction chart

X-axis: earnings dates.

Toggleable series:

- Premarket
- Event Day
- 1W

Tooltip should contain:

- date
- quarter
- EPS surprise
- revenue surprise
- corresponding return

### 3. Watchlist

Route: `/watchlist`

No authentication required in V1. Use one local/default watchlist.

Columns:

- Ticker
- Company
- Next Earnings
- Session
- Market Cap
- Sector

Allow add/remove and a watchlist-only calendar filter.

## Search

Global search should support ticker and company-name lookup, for example:

- NVDA / NVIDIA
- MU / Micron
- SNDK / Sandisk
- LEN / Lennar

Search should not be constrained to a hardcoded symbol list.

## Data architecture

Normal request path:

```text
Free external providers -> ingestion/worker -> PostgreSQL -> Go web app
```

Do not make ordinary page rendering depend on multiple live third-party API calls.

## Free data strategy

### Yahoo Finance / yfinance

Primary use:

- long-history daily prices
- recent intraday/pre-post prices
- ticker/company search
- company profiles where available
- earnings dates
- EPS estimate / reported EPS / surprise where available
- analyst/revenue estimate data where available

Because Yahoo access is unofficial and can change, isolate it behind provider interfaces and handle failures gracefully.

### SEC EDGAR

Primary use for U.S. issuers:

- official filings
- quarterly/annual financial statement data
- revenue
- company metadata
- validation of reported figures

### Massive Free

Supplementary only, within free-tier limits:

- reference/ticker data
- recent minute aggregates
- free historical market data

### Alpha Vantage Free

Fallback/validation only. Respect the free daily request limit.

## Historical premarket limitation

Do not pay for old intraday history.

For older events where a reliable premarket price is unavailable:

- store NULL
- display `N/A`

From the date the system starts running, save available recent premarket observations into PostgreSQL so the application gradually builds its own permanent history.

## Suggested database model

### companies

- id
- symbol
- name
- exchange
- country
- sector
- industry
- description
- logo_url
- market_cap
- currency
- active
- created_at
- updated_at

### earnings_events

- id
- company_id
- fiscal_year
- fiscal_quarter
- report_date
- report_time
- session (`BMO`, `AMC`, `DURING_MARKET`, `UNKNOWN`)
- eps_estimate
- eps_actual
- eps_surprise
- eps_surprise_pct
- revenue_estimate
- revenue_actual
- revenue_surprise
- revenue_surprise_pct
- source
- created_at
- updated_at

### quarterly_financials

- id
- company_id
- fiscal_year
- fiscal_quarter
- period_end
- revenue
- diluted_eps
- source
- created_at
- updated_at

### earnings_reactions

- id
- earnings_event_id
- previous_close
- premarket_price
- premarket_return_pct
- event_open
- event_close
- event_day_return_pct
- return_1d
- return_2d
- return_1w
- return_2w
- return_1m
- return_3m
- calculated_at
- methodology_version

### watchlists

- id
- name
- created_at

### watchlist_companies

- watchlist_id
- company_id

### provider_sync_state

- provider
- resource
- last_sync
- status
- error

Add sensible indexes and uniqueness constraints.

## Worker jobs

### Company sync

Refresh supported company metadata as needed.

### Earnings calendar sync

Fetch a window covering recent days through at least the next 14 days and upsert events.

### Fundamentals sync

Refresh quarterly revenue/EPS and company profile data for relevant companies.

### Earnings results sync

After reports occur, update actual EPS/revenue and surprise fields where free reliable data is available.

### Reaction calculation

Calculate event-session and forward returns idempotently.

### Forward-return update

Populate return horizons when they become available:

- 1 session -> 1D
- 2 -> 2D
- 5 -> 1W
- 10 -> 2W
- 21 -> 1M
- 63 -> 3M

## Time and market logic

For U.S. stocks use `America/New_York` explicitly.

- AMC reports react in the next valid trading session.
- BMO reports react in the same trading session.
- Respect weekends and exchange holidays.

Do not use naive `date + N days` logic for trading-session returns.

## Reliability requirements

- API timeouts
- bounded exponential retries
- provider rate limiting
- structured logging
- idempotent workers
- graceful missing-data handling
- no raw provider JSON leaking into domain/UI code
- no API keys in frontend code
- no fake financial values

## V1 definition of done

V1 is useful when a user can:

1. open the current weekly earnings calendar;
2. find companies by ticker/name;
3. open a stock page;
4. read the company description and next earnings information;
5. inspect quarterly revenue/EPS charts;
6. inspect historical EPS/revenue surprise data where available;
7. inspect event-day and forward-return history;
8. inspect premarket data where free/reliably available, otherwise `N/A`;
9. inspect summary statistics and conditional earnings behaviour;
10. add/remove companies from a local watchlist.
