# Historical quarter and revenue-surprise audit

Completed 19 September 2026. This change is limited to fiscal-period mapping and historical revenue enrichment. No paid dependency, Python, universe-strategy change, or reaction-calculation change was introduced.

## 1. Quarter coverage before and after

The frozen sample contains 58 eligible companies and 2,109 historical earnings events with report dates **before 18 September 2026**. The same events and denominator are used before and after. The sample extends the existing representative-universe audit; it is not restricted to development seed symbols. It includes foreign issuers and non-calendar fiscal years. These are sample results, not an extrapolation to the whole universe.

| Field | Before | After | After coverage |
|---|---:|---:|---:|
| Fiscal quarter | 346 (16.4%) | 1,889 | 89.6% |
| Reported revenue | 0 (0%) | 1,594 | 75.6% |
| Historical revenue estimate | 0 (0%) | 96 | 4.6% |
| Revenue surprise | 0 (0%) | 85 | 4.0% |

Quarter labels increased by 1,543 events, or 73.2 percentage points. Baseline revenue counts refer to persisted earnings-event fields, not to the separate financials table.

Evidence: [frozen sample](fiscal-revenue-sample.json), [before](fiscal-revenue-before.json), [after and source counts](fiscal-revenue-after.json).

## 2. Revenue-surprise coverage

There are now 85 sourced surprises, up from zero. The implementation stores `(actual - estimate) / abs(estimate)` as a decimal fraction; presentation multiplies by 100. A zero or missing estimate produces no surprise. Actual and estimate provenance are stored independently.

Coverage remains substantially incomplete. The optional free Alpha Vantage key worked, but this run reached the application's conservative limit of 24 requests per rolling 24 hours. Ten companies have both endpoint responses cached; two have partial responses. Unfetched companies must not be interpreted as companies for which the provider has no data. Additional scheduled refreshes can increase coverage as quota becomes available, but no particular increase is promised.

## 3. Exact fiscal mapping algorithm

1. Fetch SEC Company Facts and current plus archived submission metadata covering the historical window. Persist fiscal evidence rather than attempting to reconstruct it on page loads.
2. Accept `10-Q`, `10-Q/A`, `10-K`, and `10-K/A`. Anchor facts to their accession's report date and matching period end, excluding comparative periods. Explicit `Q1`, `Q2`, and `Q3` from quarterly filings become `sec_explicit`.
3. Map `FY` to Q4 only for an annual filing with a matching accession, fiscal year, period end, and an actual annual fact spanning 330–400 days. Record `sec_annual_q4`. Conflicting labels for an accession or period, or reuse of the same fiscal-year/quarter label for different ends, mark evidence ambiguous.
4. Include persisted financial periods as fallback evidence. Yahoo labels use explicitly reported fiscal-quarter metadata and matching financial period ends, never calendar-month assumptions. SEC evidence has priority over nearby financial-period aliases.
5. Infer a single interior missing label only when two surrounding known labels are exactly two fiscal quarters apart and both adjacent period gaps are 75–105 days. Ambiguous evidence is excluded. Record `inferred_fiscal_sequence`; do not infer financial amounts.
6. For each past earnings event, consider earlier period ends normally 20–120 days away. Filing-backed candidates require filing dates within 45 days of the event. A 7–19-day gap is permitted only with strong SEC evidence filed within 30 days, or a directly labeled financial period. Nothing outside 7–120 days matches.
7. Rank filing-backed candidates by filing-date proximity and unfiled financial periods by period-end proximity. The best candidate must beat the next candidate by more than seven days; otherwise leave the mapping unresolved. Respect existing period identity unless stronger evidence supports a correction, including a bounded seven-day period alias.
8. Preserve stronger existing mappings: explicit SEC/annual evidence outranks Yahoo event labels and legacy SEC same-day mappings, which outrank financial-period matches, which outrank sequence inference. Newly contradictory evidence can withhold an unsafe existing label. Optimistic update conditions protect concurrent changes.
9. If a corrected period association invalidates dependent revenue fields, clear those fields and re-enrich using the correct period. Actual revenue prefers directly reported SEC quarterly values, then Yahoo. No annual-minus-YTD reconstruction is performed. Yahoo date aliases require matching explicit fiscal year/quarter, a unique financial record, and a gap of at most seven days.

The idempotent backfill runs after financials, filings, and earnings synchronization, including `sync-all`. It is also available as `worker backfill-quarters SYMBOL`. A second database-only pass across all 58 companies left the fiscal/revenue fingerprint unchanged: `b8de58ed008831c591f7edd062391e25`.

## 4. Historical estimate sources discovered

### Yahoo

Live structured-response probes covered NVDA, ADBE, COST, and ASML: `earningsHistory`, `earnings`, `earningsTrend`, `calendarEvents`, and `incomeStatementHistoryQuarterly`, plus visualization and fundamentals-timeseries requests. No historical revenue consensus was found in these responses. Earnings history supplied EPS; earnings/financials structures supplied actuals and some explicit fiscal labels. Forward consensus in trend/calendar structures was deliberately rejected for historical enrichment.

The visualization EPS-estimate control succeeded; tested revenue estimate/actual/difference/surprise/consensus field variants returned HTTP 400. The timeseries probe returned quarterly total revenue, not the requested consensus/surprise candidates. This is a result for the tested accessible endpoints, not proof that Yahoo can never expose another structure. See [source audit](revenue-source-audit.json); the opt-in Go provider probe is repeatable.

### Alpha Vantage Free

The configured free key successfully retrieved `EARNINGS_ESTIMATES` and `EARNINGS`. The [official endpoint documentation](https://www.alphavantage.co/documentation/#earnings-estimates) describes the estimates endpoint. Live cached responses, rather than the endpoint name alone, establish its usefulness here.

Only closed fiscal-quarter estimates are considered. Each estimate must join the historical earnings response by exact fiscal period end, have a real reported EPS and past report date, and agree with that response's historical EPS consensus at cent precision. The resulting observation must match the persisted event's symbol, fiscal year, quarter, period end, and report date. Annual, forward, duplicate, malformed, conflicting, and unmatched observations are rejected.

This is a provider-supplied historical quarterly consensus record corroborated against its earnings history. It is not a locally captured pre-release snapshot or a guarantee about the timestamp of every analyst revision. No observation timestamp is invented. Generic estimate snapshots require a genuine observation time before the report date.

The provider is isolated behind an interface, optional, worker-only, and cached in PostgreSQL for seven days, with negative caching for unavailable history. Every HTTP attempt reserves quota atomically in PostgreSQL: at most 24 requests in a rolling day, with inter-request spacing and bounded retries. Exhausted quota is recorded as `not_attempted`, not a failed financial refresh. Page loads read persisted data. Normal scheduling prioritizes tracked companies, and the audit refresh prioritized upcoming earnings.

## 5. Coverage by provider

All percentages below use the same 2,109-event denominator. Source counts are mutually exclusive stored provenance, so combined counts do not double-count events.

| Field/source | Events | Coverage |
|---|---:|---:|
| Quarter: SEC explicit | 1,432 | 67.9% |
| Quarter: SEC annual Q4 | 432 | 20.5% |
| Quarter: financial period match | 19 | 0.9% |
| Quarter: inferred fiscal sequence | 5 | 0.2% |
| Quarter: retained legacy SEC same-day | 1 | <0.1% |
| Quarter: combined | 1,889 | 89.6% |
| Revenue actual: SEC | 1,543 | 73.2% |
| Revenue actual: Yahoo | 51 | 2.4% |
| Revenue actual: combined | 1,594 | 75.6% |
| Historical estimate: Yahoo | 0 | 0% |
| Historical estimate: Alpha Vantage | 96 | 4.6% |
| Historical estimate: combined | 96 | 4.6% |
| Surprise: Alpha Vantage estimate plus sourced actual | 85 | 4.0% |

## 6. Remaining N/A reasons

The 220 unresolved quarter labels are classified as: foreign issuer 100; no matching filing 67; ambiguous period 37; insufficient history 16; no SEC support 0. These are event-level classifications, not assertions that a foreign issuer has no SEC filings. Foreign filing forms do not necessarily supply the supported US quarterly fiscal evidence.

Of 2,024 events without a revenue surprise, 2,013 have no accepted historical estimate and 11 have an estimate but no matching quarterly actual. Reasons include quota-limited/unfetched history, unavailable provider history, period/report-date mismatch, conflicting consensus metadata, and absent directly reported quarterly actuals. Event-level revenue enrichment currently requires established USD reporting; foreign-currency actuals remain available in financial records but are not silently substituted into USD event fields.

For example, some COST estimate records use month-end dates that differ materially from filed fiscal ends; some also conflict with historical EPS consensus. They remain unused. ASML's available explicit recent fiscal labels improve quarter coverage, but foreign-currency records are not converted or treated as USD. Missing labels and estimates remain NULL and render N/A.

The Revenue Surprise column retains the requested tooltip: “Revenue surprise requires a historical analyst consensus estimate. Some older quarters are unavailable from free sources.”

## 7. Ten representative real companies

Counts below are populated fields out of the company's event count. Revenue fields were all zero in the baseline. Complete results for all 58 companies are in the linked JSON.

| Symbol | Events | Quarter before → after | Actual after | Estimate after | Surprise after |
|---|---:|---:|---:|---:|---:|
| NVDA | 40 | 11 → 36 | 32 | 0 | 0 |
| META | 40 | 0 → 40 | 31 | 0 | 0 |
| MU | 40 | 1 → 40 | 36 | 0 | 0 |
| LEN | 41 | 0 → 34 | 33 | 11 | 11 |
| CCL | 40 | 14 → 38 | 30 | 17 | 14 |
| PAYX | 40 | 10 → 40 | 36 | 29 | 26 |
| FDS | 40 | 1 → 38 | 35 | 18 | 18 |
| COST | 40 | 2 → 39 | 31 | 0 | 0 |
| APO | 40 | 1 → 18 | 16 | 0 | 0 |
| ASML | 40 | 0 → 4 | 0 | 0 | 0 |

## 8. Tests and builds

Passed on the final implementation:

- `gofmt -w .`
- `go vet ./...`
- `go test ./...`
- `go build ./...`
- PostgreSQL integration tests, including migration up/repeat/down/up, persisted enrichment, provenance protection, cache expiry, and rolling quota enforcement.
- `docker compose build` for app, worker, and migrate images.
- Repeated 58-company backfill with identical database fingerprint.
- Runtime checks: health, LEN, NVDA, and status pages returned HTTP 200; LEN rendered the requested revenue tooltip. App and database containers are healthy, and the scheduled worker is running.

Unit tests cover explicit SEC quarters, annual Q4 and amendments, comparative-fact exclusion, non-calendar fiscal years, bounded event matching, ambiguous evidence, stronger mapping preservation, sequence inference, historical estimate identity, forward-estimate rejection, formula/zero/missing handling, source priority, provider cache/quota behavior, and UI percent/tooltip/N/A behavior. Migrations were applied locally and the rebuilt app and scheduled worker restarted.
