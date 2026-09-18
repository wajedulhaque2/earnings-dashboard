# Tracked universe and coverage audit

The production calendar tracks up to 1,000 US-listed operating-company equities, selected from Yahoo's complete paginated equity screener. It is not a development-symbol list. Arbitrary symbols remain searchable and manually refreshable; outside-universe results are labeled.

## Eligibility

- Provider security type `EQUITY`.
- Exchange code NYQ (NYSE), NMS/NGM/NCM (Nasdaq), or ASE (NYSE American).
- Provider-reported USD market capitalization at least $1 billion.
- A supplied security name without preferred/preference, warrant, unit, rights, ETF, ETN, or fund designations.
- Rank qualifying listings by market capitalization descending, then symbol; retain the first 1,000. REITs and US-listed operating-company ADRs can qualify; foreign OTC copies cannot.

V1 uses exchange, security type and capitalization as liquidity/coverage filters. It does not claim a measured historical dollar-volume threshold. Provider classifications and security names can be imperfect; the policy is transparent and testable rather than an assertion that a screener is a definitive securities master.

Every evaluated listing retains eligibility, a reason, exchange/security attributes and the snapshot timestamp. Previous listings absent from a successful complete screen become `not_in_latest_screen`. A failed or empty screen preserves the previous universe. Changes publish atomically.

The scheduler checks the universe daily, the calendar every six hours, and eligible-company refreshes daily. Calendar ingestion queues eligible companies reporting from seven days ago through fourteen days ahead, preserving queue retry state. Current-window requests and due symbols get priority. Enrichment runs metadata → earnings → daily prices → reactions → quarterly fundamentals. Intraday ingestion remains an explicit optional command, not a core refresh dependency. Explicit manual requests may refresh outside-universe symbols.

## Repeatable audit

```sh
go run ./cmd/worker sync-universe
go run ./cmd/worker sync-calendar
go run ./cmd/audit -sample docs/audits/sample.json -refresh -out docs/audits/after.json
```

The audit creates a sample only when its file does not exist. Selection round-robins sector/capitalization strata and prioritizes current/next earnings. It selects 60 eligible companies; reports retain the exact sample and observed fiscal year ends. Reuse the same sample path for comparisons. No financial values are seeded or synthesized. Omit `-refresh` for a database-only measurement.

Each field reports per-company counts, total observations, and the percentage of sampled companies with at least one non-null observation. This last percentage is **not historical row completeness**: one fiscal label gives company coverage, even if most historical labels are unavailable. Inspect counts alongside percentages. Historical event counts exclude future report dates; other observation counts reflect stored non-null values, including provider-supplied future records. The audit includes aggregate EPS and gap behaviour with independent per-horizon sample counts.

Baseline `before.json` was generated with the original ingestion/calculation binary after refreshing all 60 companies. Opening Gap was not yet implemented, so its baseline is zero by construction—not evidence of missing daily opens. The later report uses the same symbols after the changes. Live providers may revise values between runs; differences are not automatically parser improvements.

SEC fiscal labels require an accession/report-date match to avoid assigning a later filing's year to comparative facts. Earnings labels additionally use only a unique same-day 10-Q/10-K association. Missing matches stay NULL. Revenue consensus is not required for analytics and remains unavailable when free providers do not supply it.

Status defaults to errors and recent tracked-universe/global operations. All, Errors, Unsupported and Current Universe filters retain access to historical operations and never erase last-success timestamps.

For a fundamentals-only parser/schema change, `-refresh-financials` re-ingests fundamentals across the same frozen sample and recalculates/measures coverage without repeating profile, earnings and price downloads. The report records the refresh mode. Fiscal labels are metadata, not row identity: conflicting/reused labels are withheld while directly reported numeric periods remain ingestible. The company/period-end unique key remains enforced.
