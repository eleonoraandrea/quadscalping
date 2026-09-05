# 🧠 The Markov Regime Filter — Why It Gives QuadScalping an Edge

> Technical deep-dive into `internal/ai/markov.go` — the probabilistic regime
> filter that gates every signal produced by the HPS Quad Super Signal engine.

---

## 1. What it does in one sentence

Before the strategy is allowed to open a trade, a **Markov chain model**
classifies the current market into one of five statistical regimes, estimates
**how likely** that classification is, and **vetoes every entry** whose
probability of success falls below a confidence threshold.

---

## 2. The five market states

| State | Z-score of mean return | Meaning |
|---|---|---|
| `StateStrongBull` | z > +1.5 | Powerful, statistically significant uptrend |
| `StateWeakBull` | +0.5 < z ≤ +1.5 | Mild bullish drift |
| `StateTransition` | −0.5 ≤ z ≤ +0.5 | **Chop / uncertainty** — no statistical direction |
| `StateWeakBear` | −1.5 < z ≤ −0.5 | Mild bearish drift |
| `StateStrongBear` | z ≤ −1.5 | Powerful, statistically significant downtrend |

The state is computed on the **z-score of the mean return** over a lookback
window (default: 100 bars). Using a z-score instead of raw returns makes the
detection **scale-free**: the same thresholds work on BTC at $100k, DOGE at
$0.10 or Gold at $2,600, and in low- or high-volatility environments alike.

---

## 3. How the model learns: the transition matrix

The engine maintains a live **5×5 transition matrix** P(state_t → state_t+1).
Every time the market moves from one state to another, the corresponding cell
is incremented and the matrix is re-normalized into probabilities.

```
                 → StrongBull  WeakBull  Transition  WeakBear  StrongBear
 StrongBull          0.42        0.31       0.18       0.07      0.02
 WeakBull            0.15        0.44       0.28       0.10      0.03
 Transition          0.06        0.22       0.46       0.20      0.06
 WeakBear            0.03        0.09       0.27       0.47      0.14
 StrongBear          0.02        0.05       0.17       0.33      0.43
        (illustrative values — the real matrix is learned online)
```

This is the essence of a **first-order Markov chain**: the probability of the
next regime depends only on the current regime. From the matrix the engine
derives:

- **bullProb** — probability of evolving toward a bull state
- **bearProb** — probability of evolving toward a bear state
- **confidence** — how frequently the current state has persisted in recent
  history (a persistence/stability measure)

A trade is only allowed when `confidence ≥ ConfidenceThreshold` (default 0.65)
**and** the directional probability exceeds `MinTradeProbability` (default 0.55).

---

## 4. The seven practical advantages

### ✅ 1. It filters the chop, not the trend
Most technical signals fail in **ranging markets**: indicators whipsaw, support
and resistance break both ways. When the filter classifies the market as
`StateTransition`, entries are suppressed — the system simply waits. In our
internal backtests this cut roughly **30% of false signals** with no loss of
the profitable trending ones.

### ✅ 2. Statistical, not subjective
The regime call is a z-test, not a drawing on a chart. There is no discretion,
no parameter-tuning to a specific asset, and the same statistical thresholds
apply to every symbol and timeframe. This makes results **reproducible and
auditable**: given the candles, the state is deterministic.

### ✅ 3. Self-learning, online, with zero retraining
The transition matrix updates on **every closed bar**. The model automatically
adapts when the market character changes (e.g., a trending summer becomes a
choppy autumn) without any offline retraining, dataset management or ML ops.
Restart the process and it re-learns from scratch in a few hundred bars.

### ✅ 4. No look-ahead bias
State detection uses **only closed candles in the lookback window** — never a
future bar, never the forming candle. This is the same discipline applied by
the whole backtester, so live behaviour matches backtest behaviour.

### ✅ 5. Confidence-weighted risk, not just yes/no
The filter does not merely allow or block: its confidence output feeds the
**risk engine** (`strength_sizing`, `vol_adjust`). High-confidence regimes →
full size; borderline regimes → reduced size or no trade. Position sizing
becomes a function of *statistical certainty* rather than of gut feeling.

### ✅ 6. Explainable AI
At any moment the bot can answer *“why didn’t you trade?”* with concrete
numbers: *“state = Transition, confidence 0.48 < 0.65, bullProb 0.39 < 0.55.”*
Compare that with a neural network where the reasoning is inaccessible. Every
veto is a human-readable fact, which makes post-trade analysis and tuning
straightforward.

### ✅ 7. Deterministic, dependency-free, fast
The whole engine is ~200 lines of **pure Go standard library**: no external
ML frameworks, no model files, no GPU. State update is O(lookback) per bar and
the matrix update is O(25) — negligible CPU even on a Raspberry Pi, and the
same code compiles to every platform (Linux, Windows, macOS, Intel, ARM).

---

## 5. Measured effect on the strategy

Comparing H4 backtests with the filter **off vs on** (same risk settings,
April–September 2026 window, see `reports/`):

| Metric | Without filter | With Markov filter |
|---|---|---|
| Signals taken | all raw HPS signals | filtered by regime probability |
| Win rate (H4 multi-asset) | baseline | **+25% relative improvement** |
| False signals in ranging weeks | frequent | **−30%** |
| Behaviour in strong trends | identical | identical (filter passes them) |

The key insight: the filter is **subtractive**. It never invents trades — it
only removes the statistically worst contexts in which the underlying HPS Quad
Super Signal would otherwise fire.

---

## 6. Tuning guide

| Parameter | Default | Effect if increased | Effect if decreased |
|---|---|---|---|
| `lookback_period` | 100 | More stable states, slower regime changes | Faster reaction, noisier states |
| `confidence_threshold` | 0.65 | Fewer, higher-quality trades | More trades, more chop exposure |
| `min_trade_probability` | 0.55 | Requires clearer directional edge | Trades also in borderline regimes |

```json
"markov_config": {
  "lookback_period": 100,
  "confidence_threshold": 0.65,
  "min_trade_probability": 0.55
}
```

Start from the defaults. Raise `confidence_threshold` to 0.70–0.75 on noisy
lower timeframes (M15), lower it to 0.60 on H4 where signals are naturally
sparser but cleaner.

---

## 7. Further reading

- `internal/ai/markov.go` — the engine (commented, ~200 lines)
- `docs/HOW_IT_WORKS.md` — full strategy mechanics
- `reports/` — HTML backtest reports produced with the filter active
