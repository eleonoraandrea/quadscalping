# Quadscalping Public Repo — Design (2026-09-04)

## Goal
Publish `quadscalping` as a public GitHub repo with full English documentation,
detailed how-it-works explanation, MIT license, and Kerben.trade affiliation (ID `EZIO`).

## Context
Existing private folder `/mnt/lavoro/trading/quadscalping` is a Go (stdlib-only)
automated trading system + backtester + HTML report for HPS Quad Super Signal.
Current README is Italian, no git, `config.json` contains secrets, `data/*.csv`
and `reports/` are local-only.

## Approach Chosen: B — Full EN docs
Rejected A (minimal, too thin for trust/affiliation) and C (CI+DOCKER+bridge,
too much scope for first public push).

## Design

### 1. Repo layout
```
quadscalping/
  README.md (EN, badges, how-it-works, results, Kerben section)
  LICENSE (MIT)
  .gitignore (config.json, data/*.csv, reports/*.html, build/, .env, state.json)
  go.mod / Makefile / cmd/ / internal/ (unchanged)
  config.example.json (sanitized)
  docs/HOW_IT_WORKS.md (deep dive EN)
  docs/AFFILIATION.md (Kerben proposal EN)
  .github/workflows/ci.yml (go vet + go test -race)
```

### 2. README structure (EN)
Badges + tagline, Kerben banner with `https://kerben.trade/?ref=EZIO`,
pipeline diagram (text), entry/exit rules, quickstart, config table,
honest 2-year 4h results, why-not-15m/1h, risk, disclaimer.

### 3. Affiliation
Top banner + `Trade live on Kerben` section + `docs/AFFILIATION.md`.
Affiliate link `https://kerben.trade/?ref=EZIO`, code `EZIO`, clear disclosure.
No connector code change in this phase; Binance spot/testnet stays default.
Future: native Kerben connector if API is Binance-compatible.

### 4. Safety
Never commit `config.json`, `data/`, API keys, Telegram tokens.
`.gitignore` + pre-commit `git status` check.

### 5. Verification
`go vet ./...`, `go test ./... -race`, `git status` clean except intended files.

## Self-review
- No TBD placeholders.
- Architecture matches code (strategy.go, indicator.go, backtest.go, risk.go, bot.go verified).
- Scope is single public-push, no unrelated refactoring.
- No ambiguity: link format `?ref=EZIO` confirmed by user.
