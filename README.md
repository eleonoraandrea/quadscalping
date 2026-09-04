# quadscalping — HPS Quad Super Signal (Go)

[![Go](https://img.shields.io/badge/Go-1.24-blue)](https://go.dev)
[![Stdlib only](https://img.shields.io/badge/deps-stdlib_only-green)](go.mod)
[![CI](https://img.shields.io/badge/CI-go_vet_%2B_test-lightgrey)](.github/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow)](LICENSE)

Automated trading bot, backtester, and self-contained HTML reports for the
**HPS Quad Super Signal** system — rewritten in Go (standard library only,
zero dependencies).

> **Trade live on Kerben:** https://kerben.trade/?ref=EZIO — affiliate ID **`EZIO`**.
> See [docs/AFFILIATION.md](docs/AFFILIATION.md) for the full proposal and disclosure.

## How it works

```
Binance public klines -> Indicators -> Strategy -> Backtest / Live bot -> Report
```

1. **Indicators** (`internal/indicator`): EMA 20/50/200, four Stochastics
   (9/14/44/60 with %D 3/3/9/10), ATR(14), rolling VWAP, volume average.
   All series are aligned; warmup bars are `NaN`.
2. **Strategy** (`internal/strategy`, lookahead-safe, closed bars only):
   mean-reversion long in **DOWN regime** (`EMA20<EMA50<EMA200` and
   `price<VWAP`) when **all four %K** are below threshold on the previous bar
   (**quad rotation**) and fast %K **crosses above** its %D (**hook**, epsilon-guarded).
   Signal strength 0–100 (regime + quad depth + hook margin + volume).
3. **Risk** (`internal/risk` + `internal/backtest` money management):
   `size = equity * risk_pct / (entry-stop)`, spot cap `MaxLeverage=1.0`,
   optional vol-adjust / strength sizing / drawdown throttle / chandelier
   trailing / loss-streak pause, plus daily loss stop and max positions.
4. **Execution** (`internal/connector`): `Paper` (simulated fills on real prices)
   or Binance spot/testnet (HMAC-signed). Default is **testnet + dry-run**.
5. **Report** (`internal/report`): single-file HTML with SVG equity, drawdown,
   monthly PnL, trade table, dark mode. Alerts via `internal/telegram` (optional).

Deep dive: [docs/HOW_IT_WORKS.md](docs/HOW_IT_WORKS.md).

### Entry / exit rules

- **Entry:** DOWN regime + quad rotation (bar `i-1`) + hook (bar `i`) +
  `strength >= min_signal_strength`. Entry at close.
- **Stop:** `low - StopATR * ATR` (`atr` mode) or swing-low minus buffer.
- **Take profit:** partial `PartialPct` (default 50%) at `TP1R` (default 1R),
  stop moved to breakeven; optional `TP2R` final target.
- **Slow exit:** long %K crosses below its %D above `ExitSlow`.
- **Guards:** warmup bars, cooldown bars after exits, forced close at end of data
  in backtests, entry fees included in PnL.

## Quickstart

```bash
# 1. Download history (public endpoint, no API key)
go run ./cmd/download -symbol BTCUSDT -interval 5m -days 365

# 2. Backtest + HTML report
cp config.example.json config.json
go run ./cmd/backtest -config config.json -out reports/report.html

# 3. Open the report
xdg-open reports/report.html
```

### Paper / dry-run bot

```bash
go run ./cmd/hpsbot -config config.json
# status: http://localhost:8080/status
```

Default is **testnet + dry-run** (simulated orders on real prices). To go live:
set `exchange.dry_run:false`, `testnet:false`, and provide `BINANCE_API_KEY` /
`BINANCE_API_SECRET` (or config fields). Always test on testnet first, then
paper-trade for weeks. Never commit `config.json` with real keys.

### Telegram (optional)

1. Create a bot with @BotFather → token.
2. Get chat id from @userinfobot.
3. In `config.json`: `"telegram": {"enabled": true, "bot_token": "...", "chat_id": "..."}`
   or env `TELEGRAM_BOT_TOKEN` / `TELEGRAM_CHAT_ID`.

## Configuration

See `config.example.json`. Key fields:

| Section | Field | Meaning |
|---|---|---|
| `trading` | `symbols`, `timeframe` | e.g. BTC/ETH/DOGEUSDT @ 4h |
| `trading` | `risk_pct` | Risk per trade (fraction of equity) |
| `trading` | `max_daily_loss_pct` | Daily stop for the bot |
| `hps` | `stop_atr`, `tp1_r`, `partial_pct` | Stop multiple, TP1 R, partial fraction |
| `hps` | `exit_slow`, `min_signal_strength` | Slow-exit threshold, min strength 0–100 |
| `hps` | `vwap_period`, `regime_filter` | Rolling VWAP window, entry regime |
| `risk` | `vol_adjust`, `strength_sizing`, `dd_throttle_pct`, `trail_atr_mult`, `loss_streak_n` | Money management toggles |
| `bot` | `poll_interval_sec`, `commission`, `slippage` | Loop cadence, paper costs |
| `symbol_overrides` / `risk_overrides` | per-symbol | Winners use 4h per-symbol params |

## Improvements over the Python prototype

- **Rolling VWAP** (configurable window) instead of cumulative over the full
  series — cumulative is unusable live and distorts results.
- **Compound sizing on equity**, not on initial capital.
- **Sharpe from per-bar equity returns**, annualized (Python used per-trade PnL
  with `sqrt(288)`, which is meaningless).
- **Breakeven after partial TP** (configurable).
- **Cooldown bars** after exits to avoid rapid re-entries.
- **Leverage cap** (default 1x = spot): risk-percent sizing with tight stops
  produced unbounded notionals.
- **Epsilon-guarded hook**: floating-point noise crossings are not signals.
- **Forced close at end of data** (Python dropped the open position).
- **Entry fees included** in both bot and backtest PnL.

## Tuning and honest results (September 2026)

The original Python config (5m, 1.5 ATR stop, 1.5R TP1) on BTCUSDT lost money:
PF 0.66, -$5,910, 60% maxDD over 12 months. Key diagnostic: **gross** edge was
already ~zero (no-fee PF 0.99) — costs (~0.25R per trade with tight 5m stops)
only made it worse.

Grid search (`cmd/sweep`): 648 combos x 3 timeframes, robustness validated on
**3 independent thirds** (profitable in all 3, >=8 trades per third).
Recommended: **4h** with per-symbol params (`symbol_overrides`).

### Money management

`risk` section (+ per-symbol `risk_overrides`), compared with
`go run ./cmd/sweep -risk -symbol ETHUSDT -intervals 4h`:

- **vol_adjust** — smaller size when ATR is above its average (clamp 0.5–1.25)
- **strength_sizing** — size x (0.5 + 0.5*strength/100)
- **dd_throttle_pct** — risk scaled by drawdown (0.25x floor)
- **trail_atr_mult** — chandelier trailing after TP1
- **loss_streak_n** — pause after N consecutive losses

Chosen (both robust 3/3 thirds): BTC = strength_sizing (Calmar 1.23→1.26,
DD 5.1→4.9%); ETH = vol_adjust (Calmar 1.76→3.13, PnL +714→+996, DD 4.0→3.2%).

### Final results (2 years, 4h, net of fees+slippage)

| Symbol | Trades | Win | PF | PnL | maxDD | Sharpe |
|---|---|---|---|---|---|---|
| BTCUSDT | 35 | 68.6% | 1.54 | **+$618 (+6.2%)** | 4.9% | 0.83 |
| ETHUSDT | 33 | 54.5% | 1.71 | **+$996 (+10.0%)** | 3.2% | 1.15 |
| DOGEUSDT | 34 | 50.0% | 1.10 | +$157 (+1.6%) | 5.7% | 0.22 |
| **total** | **102** | | | **+$1770 (+17.7%)** | | |

### Why not 15m / 1h

Tested and rejected with the same 3-thirds rule:
**15m** — 216 combos x BTC/ETH: zero robust. No-fee diagnostic PF 0.98/0.90,
so gross edge is ~zero even at zero cost; ~0.1%/round-trip costs only worsen it.
More 15m trades = more losses, not more edge. **1h** — zero robust combos on both.
**Rejected at 4h**: BNB (0 robust), SOL (thirds ok but full-sample negative),
XRP (third PFs 1.07–1.15, too marginal).

The edge of this system on this data lives at 4h with wide stops (2–3 ATR).
Frequency is ~1 trade/week/symbol: to trade more, add robust symbols, not lower
timeframes.

Honest caveats: 216 combos x 6 symbols tested, 3 kept — selection bias is
possible; thirds are ~8 months each. Re-tune periodically:

```bash
go run ./cmd/sweep -symbol BTCUSDT -intervals 1h,4h -thirds
go run ./cmd/sweep -risk -symbol ETHUSDT -intervals 4h
```

## Trade live on Kerben

Recommended execution venue: **[Kerben.trade — register with ID EZIO](https://kerben.trade/?ref=EZIO)**.

Affiliate disclosure: this is an affiliate link (`EZIO`). If you register and
trade, the maintainer may earn a commission at no extra cost to you.
Full proposal: [docs/AFFILIATION.md](docs/AFFILIATION.md).

Paper-trade first, then use Kerben spot with trade-only API keys. Backtests are
net of `0.04%` commission + `0.01%` slippage per side — compare with Kerben's
real fees before sizing up.

## Tests

```bash
make test    # go test ./... -race
make vet
```

## Project structure

```
cmd/hpsbot      live/paper bot
cmd/backtest    backtest -> HTML report
cmd/download    Binance klines downloader
cmd/sweep       parameter + risk grid search
internal/...    indicator, strategy, backtest, data, connector, risk,
                telegram, report, bot, config, market
docs/...        HOW_IT_WORKS, AFFILIATION
```

## Disclaimer

Educational/research software. Trading involves significant risk; backtest
results do not predict future performance. Not financial advice.
