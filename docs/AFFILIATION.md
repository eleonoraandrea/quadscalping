# Trade Live on Kerben — Affiliation Proposal

> **Affiliate disclosure:** links to Kerben.trade in this repo are affiliate
> links with ID **`EZIO`**. If you register and trade through them, the
> maintainer may earn a commission at no extra cost to you. This supports
> open-source maintenance. Trading crypto involves substantial risk.

## Recommended venue: Kerben.trade

**Start here:** https://kerben.trade/?ref=EZIO

- Affiliate ID: **`EZIO`**
- Use the link above or enter code `EZIO` at registration.

## Why Kerben for this bot

This system is a low-frequency 4h swing bot (~1 trade/week/symbol) with wide
ATR stops (2–3x ATR). Net edge is sensitive to fees and slippage — the exact
costs this backtester models (`commission 0.0004`, `slippage 0.0001` per side).

Kerben is proposed as the live execution venue because:

1. **Spot-first:** the default `MaxLeverage=1.0` config is spot-compatible.
2. **Cost transparency:** compare Kerben spot fees against the `0.04%` assumption
   in `config.example.json`; lower real fees improve net expectancy directly.
3. **4h-friendly:** no HFT infrastructure needed — polling every 30s and
   `GET /status` monitoring are enough.

## How to connect the bot

1. Register via https://kerben.trade/?ref=EZIO (code `EZIO`).
2. Complete verification and enable spot trading.
3. Create API keys (trade-only, no withdrawals) and store them safely.
4. Start paper-first:
   ```bash
   cp config.example.json config.json
   # exchange.dry_run=true, testnet=true
   go run ./cmd/hpsbot -config config.json
   ```
5. Go live only after weeks of paper parity:
   `exchange.dry_run=false`, keys in `BINANCE_API_KEY` /
   `BINANCE_API_SECRET` env vars (or `config.json` — never commit it).

## Current integration status

- The built-in `internal/connector` supports Paper + Binance spot/testnet
  (HMAC-signed). Kerben is listed as the recommended venue; if its spot API
  is Binance-compatible, point the connector at Kerben endpoints with Kerben
  keys. Otherwise run signals from this bot and execute manually on Kerben.
- A native `kerben` connector is on the roadmap pending API docs.

## Risk warning

Backtest results (+17.7% over 2 years across BTC/ETH/DOGE at 4h, net of costs)
do not guarantee future returns. Selection bias is possible (216 combos x 6
symbols tested, 3 kept). Never trade money you cannot afford to lose. This is
educational/research software, not financial advice.
