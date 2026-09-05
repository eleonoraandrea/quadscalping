# How It Works — quadscalping (HPS Quad Super Signal in Go)

This document explains in detail how the system works, end to end.
Standard library only. No external Go dependencies.

## 1. Big picture

```
Binance public klines (cmd/download, CSV cache)
  -> Indicators (internal/indicator)
  -> Strategy signals (internal/strategy)
  -> Backtester (internal/backtest) OR Live/Paper bot (internal/bot)
  -> Risk manager (internal/risk)
  -> Execution (internal/connector: Paper | Binance spot/testnet)
  -> HTML report (internal/report) + Telegram alerts (internal/telegram)
```

Two modes share the same signal code:

- **Backtest** (`cmd/backtest`): replays historical candles, simulates fills with
  commission + slippage, produces metrics and a self-contained HTML report.
- **Bot** (`cmd/hpsbot`): polls closed candles every `poll_interval_sec`,
  manages open positions, persists state to `data/state.json`, exposes
  `GET /status`.

A parameter sweeper (`cmd/sweep`) searches strategy and risk grids and validates
robustness on 3 independent thirds.

## 2. Data — `internal/data`, `internal/market`, `cmd/download`

- Source: Binance public klines REST, no API key required.
- `cmd/download -symbol BTCUSDT -interval 5m -days 365` paginates, respects
  rate limits, and maintains an incremental CSV cache in `data/`.
- `market.Candle{Time, Open, High, Low, Close, Volume}` is the single
  canonical type. `market.Minutes("4h")` converts timeframe strings for
  Sharpe annualization and pause math.
- All strategy evaluation is **lookahead-safe**: signals are computed on
  closed candles only (`EvaluateLast` = last closed bar).

## 3. Indicators — `internal/indicator`

All series are aligned to input length; warmup bars are `NaN`.

- `EMA(closes, 20/50/200)`: exponential moving average seeded with the SMA of
  the first `p` values.
- `StochK(highs, lows, closes, n)`: `100*(C-LL)/(HH-LL)` over `n` bars.
  Flat window (`HH==LL`) returns `50` (neutral).
- `StochD(k, d)`: SMA of `%K`. Skips leading `NaN` so warmup starts at the
  first valid `%K`.
- Four stochastics: fast `(9,3)`, small `(14,3)`, mid `(44,9)`, long `(60,10)`.
- `ATR(highs, lows, closes, 14)`: SMA of true range. `TR[0]=NaN`.
- `RollingVWAP(highs, lows, closes, volumes, n)`: windowed VWAP with
  `TP=(H+L+C)/3`. Falls back to SMA of TP when window volume is zero.
  This replaces the cumulative VWAP of the Python prototype, which is
  unusable live and distorts late samples.
- `SMA(volumes, 20)`: 20-bar average volume for the strength bonus.

## 4. Strategy — `internal/strategy`

Mean-reversion long system. File: `strategy.go`, `Compute` + `Evaluate`.

### 4.1 Regime filter (`RegimeAt`)

- `UP`: `Close > EMA200 && EMA20 > EMA50 > EMA200 && Close > VWAP`.
- `DOWN`: `Close < EMA200 && EMA20 < EMA50 < EMA200 && Close < VWAP`.
- Else `SIDE`.
- Default `RegimeFilter="down"`: entries only in `DOWN` (buy the dip in a
  downtrend when momentum is washed out).

### 4.2 Quad rotation + hook (entry)

Evaluated at closed bar `i`:

1. **Quad rotation** on bar `i-1`: all four `%K` below thresholds
   (`QuadFast 9`, `QuadSmall 14`, `QuadMid 44`, `QuadLong 60`; defaults
   `30/30/30/40`, overridable per symbol).
2. **Hook** on bar `i`: fast `%K` crosses above its `%D`
   (`Kf[i]-Df[i] > eps && Df[i-1]-Kf[i-1] > -eps`, `eps=1e-6` to ignore
   floating-point noise).
3. Regime must match the filter.
4. `WarmupBars = max(EMA200, VWAPPeriod, StopLookback)+10` must have passed.

### 4.3 Levels

- `entry = Close[i]`.
- Stop (`stopPrice`):
  - `atr` mode (default): `Low[i] - StopATR * ATR[i]`.
  - `swing` mode: lowest low of last `StopLookback` bars minus
    `StopBufferATR * ATR[i]`.
- `TP1 = entry + TP1R * (entry-stop)`, `TP2 = entry + TP2R * risk` (`0`=off).
- Reject if `stop >= entry`.

### 4.4 Signal strength 0–100 (`strength`)

- Base `40` (quad + hook happened).
- Regime: `+25` if favored, `+5` if `SIDE`.
- Quad depth: `+0..20` (how far the four `%K` were below thresholds).
- Hook margin `Kf-Df`: `+0..10`.
- Volume: `+5` if `Volume > VolAvg20`.
- Clamped to `100`. Entry requires `strength >= MinStrength`.

### 4.5 Slow exit

Bearish cross of the long stochastic above `ExitSlow`:
`Kl[i] < Dl[i] && Kl[i-1] >= Dl[i-1] && Kl[i] > ExitSlow`.
Checked before entry logic on every bar, so an open position can be closed
on the same evaluation path.

## 5. Backtester — `internal/backtest`

`Run(symbol, candles, cfg)` is the reference simulator.

- Starts at `warmup`, entry at `Close` + slippage, management from the next bar.
- Priority per bar when holding: **stop first** (conservative), then partial TP1,
  then TP2, else slow exit on close.
- Partial TP1: sells `PartialPct` (default 50%), moves stop to breakeven if
  `BreakevenOnTP1=true`.
- Optional chandelier trailing after TP1: `stop = max(stop, highSince - TrailATRMult*ATR)`,
  applied end-of-bar so it acts from the following bars.
- `CooldownBars` after any exit prevents rapid re-entry.
- **Compound sizing on equity**: `size = equity * RiskPct / (entry-stop)`,
  multiplied by money-management factors, capped by `MaxLeverage` (default
  `1.0` = spot, no leverage). Entry fee is debited immediately.
- Forced close at end of data (`END_OF_DATA`) so open positions are never
  silently dropped.
- Costs: `Commission 0.0004/side`, `Slippage 0.0001/side` by default.

### 5.1 Money management (`sizeFactor`)

- `vol_adjust`: scales by `clamp(avgATR/ATR, 0.5, 1.25)` — smaller in high vol.
- `strength_sizing`: scales by `0.5 + 0.5*strength/100`.
- `dd_throttle_pct`: scales down with drawdown, floor `0.25x`.
- `loss_streak_n` + `loss_streak_pause_bars`: pauses entries after N
  consecutive losses.
- Chosen after `cmd/sweep -risk` grid search (see README results):
  BTC = `strength_sizing`, ETH = `vol_adjust`.

### 5.2 Metrics (`ComputeMetrics`)

Win rate, profit factor, net PnL, expectancy, payoff, best/worst, max
drawdown ($ and %), SQN, exposure, fees paid, and **Sharpe from per-bar equity
returns, annualized** (`barsPerYear = 525600/timeframeMinutes`). This fixes the
Python prototype, which computed Sharpe from per-trade PnL with `sqrt(288)`.

## 6. Live / paper bot — `internal/bot`, `cmd/hpsbot`

- `Cycle()`: first manages open positions, then scans flat symbols.
- `scanSymbol`: fetches candles via injectable `Fetcher`, runs
  `Compute` + `EvaluateLast`, calls `openPosition` on `BUY_ENTRY`.
- `openPosition`: checks `risk.CanOpen` (max positions, daily loss stop),
  reads live price (`LastPrice`, falls back to last close), applies the same
  sizing + `MaxLeverage` cap as the backtester, sends a `Buy` market order,
  recomputes levels from the actual fill, persists state, notifies Telegram.
- `managePosition`: stop → partial TP1 → TP2 → slow exit, same priority as
  the backtester. Loss-streak pause uses real time
  (`pauseBars * timeframeMinutes`).
- State (`data/state.json`): positions, realized PnL, daily stats, peak equity,
  consecutive losses, pause deadline. Reloaded on restart.
- `GET /status` (default `:8080`) returns JSON state for monitoring.

## 7. Execution — `internal/connector`

`Connector` interface: `Name`, `LastPrice`, `MarketOrder`, `Balance`.

- `Paper`: fills at requested price, tracks a virtual USDT balance. Used for
  `dry_run=true` on real prices.
- `Binance`: signed HMAC-SHA256 spot orders, `testnet` flag, quantity
  precision rounding. Keys from `BINANCE_API_KEY` / `BINANCE_API_SECRET`
  or `config.json`. Default is **testnet + dry_run**; set both to `false`
  for live.
- Fees: both bot and backtester apply `Bot.Commission` consistently.

## 8. Risk manager — `internal/risk`

- `PositionSize(equity, entry, stop) = equity * RiskPct / (entry-stop)`.
- `CanOpen`: blocks when `openPositions >= MaxPositions` or when
  `dailyPnL <= -startOfDay * MaxDailyLossPct`. Day rolls on UTC date change.

## 9. Report + alerts — `internal/report`, `internal/telegram`

- `internal/report`: self-contained HTML (inline SVG equity, drawdown, monthly
  PnL, trade table, dark mode). No external JS.
- `internal/telegram`: optional trade notifications. Create a bot via
  `@BotFather`, get chat id via `@userinfobot`, set
  `TELEGRAM_BOT_TOKEN` / `TELEGRAM_CHAT_ID` or the `telegram` config block.

## 10. Tuning honesty (September 2026)

- Original 5m config lost money (PF 0.66, -$5,910, 60% maxDD over 12 months
  on BTCUSDT). Gross edge without costs was already ~zero (PF 0.99), so tight
  5m stops (~0.25R cost per trade) only made it worse.
- Grid: 648 combos x 3 timeframes, robustness = profitable in **all 3
  independent thirds** with >=8 trades per third. Winner: **4h** with per-symbol
  overrides. 15m/1h produced zero robust combos; BNB/SOL/XRP were rejected at 4h.
- Final 2-year 4h net (fees+slippage, $10k capital, 1% risk):
  BTC 35 trades +$618, ETH 33 +$996, DOGE 34 +$157. Total +$1,770 (+17.7%).
  Frequency is ~1 trade/week/symbol — scale by adding robust symbols, not by
  dropping to lower timeframes.
- Caveat: 216 combos x 6 symbols tested, 3 kept — selection bias is possible.
  Re-run `cmd/sweep` periodically.
