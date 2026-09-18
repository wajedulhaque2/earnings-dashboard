# Runtime verification report — 18 September 2026

Implemented directly in this repository. No paid APIs, Python, synthetic seed values, or inferred financial observations were added.

## Universe and calendar

- Final production universe: **1,000 listings**.
- Criteria: Yahoo EQUITY; NYSE (NYQ), Nasdaq (NMS/NGM/NCM), or NYSE American (ASE); supplied security name excluding preferred/preference, warrants, units, rights, ETFs, ETNs and funds; USD market cap >= $1 billion; highest 1,000 qualifying market caps, symbol tie-break. OTC listings are excluded. Operating-company ADRs and REITs may qualify. No historical dollar-volume threshold is claimed; V1 uses the permitted exchange/security/capitalization filters.
- Eligibility, reason, source security/exchange attributes and check time are persisted. A complete snapshot publishes atomically; failed/empty screens preserve the prior selection.
- Week beginning **14 September 2026**: **239 stored calendar companies before filtering → 3 tracked companies** (FPS, TCOM, LEN). This is a filtered stored-calendar comparison, not a claim that only three companies in the market reported.
- Calendar refresh succeeded and automatically queued **18 eligible current/next-window companies**. All 18 automatically queued companies completed successfully through metadata, earnings, prices, calculations and fundamentals; the request queue was empty at final verification. No individual manual sync-all was needed. Scheduler remains configured to continue processing.

## Frozen representative sample

60 companies were selected using sector/capitalization strata with current/next earnings priority. The refreshed sample covers 10 sectors, 21 mega-cap (>= $200B), 20 large-cap ($10–200B), and 19 mid-cap ($1–10B) companies, plus 18 distinct reported fiscal year-end month/day values. The current-week reporting companies are included. Market values and classifications can change.

All 60 were eligible at selection. A later daily universe refresh moved RRC and SAIA outside the ranking; **58 remain eligible**. The same 60 names were retained for the before/after comparison rather than replacing weaker observations. See [frozen sample](sample.json), [baseline](before.json), and [final report data](after.json) for per-company counts.

The baseline refreshed all 60 with the original parser/calculation binary on 17 September. The full after-refresh ran on 18 September, followed by a fundamentals-only rerun across all 60 after fixing fiscal identity. All required sample refreshes completed. Four SEC operations still report legitimate data_unavailable for ABVX, TCOM, ASML and BNS; Yahoo fallback supplies quarterly fields. Overall refresh success does not assert success from every optional provider.

## Coverage before and after

Percentages below mean **companies with at least one non-null observation / 60**. They do not mean complete historical row coverage. Profile means a sourced company name; description has its own metric. Counts include all stored observations except earnings_events, which counts past report dates. The JSON definition records these distinctions.

| Field | Before company coverage | After company coverage | Before observations | After observations |
| --- | ---: | ---: | ---: | ---: |
| company_profile | 100.0% | 100.0% | 60 | 60 |
| daily_prices | 100.0% | 100.0% | 138289 | 138349 |
| description | 100.0% | 100.0% | 60 | 60 |
| earnings_events | 100.0% | 100.0% | 2189 | 2189 |
| eps_surprise | 100.0% | 100.0% | 2211 | 2212 |
| event_day | 100.0% | 100.0% | 2157 | 2158 |
| industry | 100.0% | 100.0% | 60 | 60 |
| market_cap | 100.0% | 100.0% | 60 | 60 |
| opening_gap | 0.0% | 100.0% | 0 | 2158 |
| quarter_labels | 98.3% | 98.3% | 59 | 464 |
| quarterly_eps | 100.0% | 100.0% | 3017 | 2988 |
| quarterly_revenue | 100.0% | 100.0% | 2860 | 2830 |
| return_1m | 98.3% | 98.3% | 2147 | 2148 |
| return_1w | 98.3% | 98.3% | 2151 | 2151 |
| revenue_surprise | 0.0% | 0.0% | 0 | 0 |
| sector | 100.0% | 100.0% | 60 | 60 |

Opening Gap was not implemented in the baseline, so its zero is by construction, not missing daily prices. Extra daily bars and small reaction/surprise changes can reflect elapsed time and provider revisions. Quarterly counts fell mainly because identical cross-source period duplicates are shown once; raw records remain stored. Higher counts are not automatically better coverage.

## Systematic fixes and remaining limitations

- Reuse Yahoo authentication cookies/crumb rather than repeating failed authentication and renewal for every protected request.
- Use daily OHLC for Opening Gap, with no intraday dependency. Existing BMO/AMC, holidays, exact-session lookup, future-horizon withholding and NULL exclusions remain.
- SEC fiscal context is accepted only for an accession whose reported period end matches the fact end. Comparative facts do not inherit a later filing label. Conflicting/reused labels are withheld.
- Removed the obsolete unique constraint on fiscal year/quarter that rejected directly reported periods for ORCL, WMS, FLS, VMI and BSY. Company/period-end uniqueness remains. The final rerun showed no refresh_failed errors for those companies.
- Same-day filing associations add earnings quarter labels only for a unique, explicitly labeled reported period. Explicit Yahoo labels retain separate provenance. Source fiscal metadata can itself be inconsistent; no calendar-month reconstruction is used.
- Canonical quarter presentation suppresses a Yahoo record only when both non-null values and currency exactly match a nearby SEC record in the same month, with both SEC field provenances verified. Raw records remain available. This avoids duplicate bars without inventing values or deleting source data.
- Yahoo company metadata, historical earnings and daily-price parsers already covered all 60 companies in the baseline; no ticker-specific parser exceptions were introduced. SEC CIK mapping remains the official general mapping.
- Weakest field: revenue surprise, **0%**. Historical revenue consensus is not reliably supplied by these free adapters; EPS analytics do not require it.
- Fiscal labels remain sparse across individual historical rows despite high company-level coverage. ABVX has no reliable label. Only a subset of SEC filings is present in the recent submissions index, and ambiguous or unmatched periods remain N/A.
- FPS has only one observed earnings event and no completed 1W/1M horizon in the audit. Missing future returns remain NULL. Unsupported currencies, IFRS/company-specific SEC concepts and unusual quarter durations can require Yahoo fallback or remain N/A.

## Opening Gap verification

Formula: event_session_open / previous_regular_session_close − 1. Unit tests verify Tuesday AMC and Wednesday BMO both use Tuesday close/Wednesday open, missing opens preserve other valid returns, and holiday/session handling is retained. Live SQL found **zero formula mismatches** among stored non-null gaps. The sample contains **2,158 Opening Gap observations across all 60 companies**. The UI displays the requested Date, Quarter, Revenue Surprise, EPS Surprise, Opening Gap, Event Day and six forward horizons. Intraday history remains stored separately.

## EPS Surprise Behaviour — pooled sample

Large groups overlap broad beat/miss groups. N counts qualifying events; each displayed return has its own non-null sample. These historical descriptive statistics are not forecasts.

| Group | N | Avg Opening Gap | Avg Event Day | Event win % | Avg 1W | Avg 1M |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| EPS Beat | 1728 | 0.94% (n=1707) | 1.07% (n=1707) | 54.72% | 0.41% (n=1702) | 1.63% (n=1699) |
| EPS Miss | 457 | -2.22% (n=447) | -2.39% (n=447) | 31.99% | -0.10% (n=445) | 0.93% (n=445) |
| Large EPS Beat (> +10%) | 741 | 2.07% (n=729) | 2.29% (n=729) | 61.87% | 0.59% (n=727) | 2.11% (n=726) |
| Large EPS Miss (< -10%) | 192 | -2.44% (n=189) | -2.52% (n=189) | 36.51% | -0.13% (n=189) | 1.07% (n=189) |

## Opening Gap Behaviour — pooled sample

| Group | N | Avg Event Day | Event win % | Avg 1W | Avg 1M |
| --- | ---: | ---: | ---: | ---: | ---: |
| Gap > +5% | 297 | 9.72% (n=297) | 96.30% | 0.88% (n=295) | 3.07% (n=293) |
| Gap +2% to +5% | 380 | 3.00% (n=380) | 77.89% | 0.22% (n=377) | 1.26% (n=376) |
| Gap -2% to +2% | 864 | 0.19% (n=864) | 50.23% | 0.43% (n=863) | 1.64% (n=863) |
| Gap -5% to -2% | 365 | -3.60% (n=365) | 14.25% | -0.10% (n=364) | 0.92% (n=364) |
| Gap < -5% | 252 | -8.46% (n=252) | 4.37% | -0.08% (n=252) | 0.18% (n=252) |

## Checks and runtime

Passed: `gofmt -w .`, `go vet ./...`, `go test ./...`, `go build ./...`, PostgreSQL integration tests with isolated migration/upsert/rollback schemas, and `docker compose build`. New checks cover Opening Gap timing/NULLs, EPS-only overlapping groups, universe filtering/ranking, authentication reuse, conservative fiscal metadata, default calendar/status filtering, idempotent automatic queueing and preservation of duplicate raw observations. Browser verification confirmed populated historical/behaviour tables, renamed charts and status filters.

Docker Desktop was initially blocked by stale Windows runtime sockets. Its socket-only directories were preserved under backup names and recreated; no factory reset or volume deletion was performed. The existing PostgreSQL database was retained. The web application is available at http://localhost:8080/calendar.
