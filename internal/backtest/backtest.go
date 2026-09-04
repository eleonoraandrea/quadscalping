// Package backtest contiene il motore di backtesting e le metriche.
package backtest

import (
	"math"

	"quadscalping/internal/indicator"
	"quadscalping/internal/market"
	"quadscalping/internal/strategy"
)

// Config configura il backtest (capitale, costi, rischio, parametri HPS).
type Config struct {
	InitialCapital float64
	RiskPct        float64 // rischio per trade frazione dell'equity
	Commission     float64 // commissione per lato (frazione)
	Slippage       float64 // slippage per lato (frazione)
	MaxLeverage    float64 // cap esposizione (1 = spot, no leva)
	CooldownBars   int     // barre di pausa dopo un'uscita prima di rientrare
	// TimeframeMinutes serve per annualizzare Sharpe/Esposizione (default 5).
	TimeframeMinutes int
	Params           strategy.Params

	// ---- money management (opzionale, default off) ----
	VolAdjust           bool    // riduci la size quando l'ATR è sopra la sua media
	VolLookback         int     // barre per la media ATR (default 100)
	StrengthSizing      bool    // scala la size con la forza del segnale (0.5..1)
	DDThrottlePct       float64 // budget di drawdown: il rischio scende col DD (0 = off)
	TrailATRMult        float64 // trailing chandelier dopo il TP1 (0 = breakeven statico)
	LossStreakN         int     // N perdite consecutive -> pausa (0 = off)
	LossStreakPauseBars int     // durata della pausa in barre
}

// DefaultConfig replica i costi di riferimento Python.
func DefaultConfig() Config {
	return Config{
		InitialCapital:      10000,
		RiskPct:             0.01,
		Commission:          0.0004,
		Slippage:            0.0001,
		MaxLeverage:         1.0,
		CooldownBars:        10,
		TimeframeMinutes:    5,
		VolLookback:         100,
		LossStreakPauseBars: 24,
		Params:              strategy.DefaultParams(),
	}
}

// sizeFactor calcola il moltiplicatore di money management per l'entry
// (vol, forza segnale, drawdown throttle).
func (cfg Config) sizeFactor(sig strategy.Signal, atr, volAvg, peakEq, eq float64) float64 {
	f := 1.0
	if cfg.VolAdjust && atr > 0 && volAvg > 0 && !math.IsNaN(volAvg) {
		v := volAvg / atr // ATR alto -> fattore < 1
		f *= clamp(v, 0.5, 1.25)
	}
	if cfg.StrengthSizing {
		f *= 0.5 + 0.5*sig.Strength/100
	}
	if cfg.DDThrottlePct > 0 && peakEq > 0 {
		dd := (peakEq - eq) / peakEq
		if dd > 0 {
			f *= clamp(1-dd/cfg.DDThrottlePct, 0.25, 1)
		}
	}
	return f
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// at ritorna xs[i] o NaN se fuori range / serie assente.
func at(xs []float64, i int) float64 {
	if xs == nil || i < 0 || i >= len(xs) {
		return math.NaN()
	}
	return xs[i]
}

// Reason di chiusura trade.
const (
	ReasonStopLoss   = "STOP_LOSS"
	ReasonTakeProfit = "TAKE_PROFIT"
	ReasonSlowExit   = "SLOW_EXIT"
	ReasonEndOfData  = "END_OF_DATA"
)

// Trade è un round-trip completo (entry, eventuale parziale, uscita finale).
type Trade struct {
	Symbol                string
	EntryTime, ExitTime   int64
	EntryIndex, ExitIndex int
	EntryPrice, ExitPrice float64
	InitialSize, Size     float64
	RiskAmount            float64
	PnL                   float64 // netto, commissioni incluse
	Fees                  float64
	R                     float64 // PnL / RiskAmount
	Reason                string
	PartialFilled         bool
	Strength              float64
}

// Metrics è il set di statistiche del backtest.
type Metrics struct {
	TotalTrades, Wins                    int
	WinRate                              float64
	GrossProfit, GrossLoss, ProfitFactor float64
	NetPnL, AvgTrade, Expectancy         float64
	AvgWin, AvgLoss, Payoff              float64
	BestTrade, WorstTrade                float64
	MaxDrawdown, MaxDrawdownPct          float64
	SQN, Sharpe                          float64
	Exposure                             float64
	FeesPaid                             float64
	BarsInMarket, TotalBars              int
}

// Result esito completo del backtest per un simbolo.
type Result struct {
	Symbol      string
	Trades      []Trade
	Equity      []float64
	EquityTimes []int64
	Metrics     Metrics
	Config      Config
}

type openPos struct {
	entryPrice, stop, tp1, tp2 float64
	size, initialSize, riskAmt float64
	entryTime                  int64
	entryIdx                   int
	realized, fees             float64
	partial                    bool
	strength                   float64
	highSince                  float64 // massimo dalla entry (per il trailing)
}

// Run esegue il backtest HPS sulle candele. Lookahead-safe:
// i segnali sono valutati su barre chiuse, l'entry avviene alla chiusura
// (con slippage), la gestione parte dalla barra successiva.
// Migliorie rispetto al motore Python: sizing composto sull'equity realizzato,
// cap di leva, breakeven dopo il TP1, chiusura forzata a fine dati.
func Run(symbol string, candles []market.Candle, cfg Config) Result {
	p := cfg.Params
	s := strategy.Compute(candles, p)
	warmup := strategy.WarmupBars(p)
	n := len(candles)

	res := Result{Symbol: symbol, Config: cfg}
	if n <= warmup+1 {
		res.Metrics = ComputeMetrics(nil, []float64{cfg.InitialCapital}, cfg.TimeframeMinutes, 0, 0, 0)
		res.Equity = []float64{cfg.InitialCapital}
		return res
	}

	var trades []Trade
	equity := []float64{cfg.InitialCapital}
	times := []int64{candles[warmup].Time}
	var realized float64 // pnl netto realizzato (fee incluse)
	var feesTotal float64
	barsInMarket := 0
	lastExitIdx := math.MinInt
	pauseUntilIdx := math.MinInt
	consecLosses := 0
	peakEquity := cfg.InitialCapital
	var pos *openPos

	var volAvg []float64
	if cfg.VolAdjust {
		volAvg = indicator.SMA(s.ATR, cfg.VolLookback)
	}

	closeAt := func(i int, price float64, reason string) {
		lastExitIdx = i
		fee := price * pos.size * cfg.Commission
		pnl := (price-pos.entryPrice)*pos.size - fee
		if pos.realized+pnl < 0 {
			consecLosses++
		} else {
			consecLosses = 0
		}
		if cfg.LossStreakN > 0 && consecLosses >= cfg.LossStreakN {
			pauseUntilIdx = i + cfg.LossStreakPauseBars
			consecLosses = 0
		}
		realized += pnl
		feesTotal += fee
		trades = append(trades, Trade{
			Symbol:        symbol,
			EntryTime:     pos.entryTime,
			ExitTime:      candles[i].Time,
			EntryIndex:    pos.entryIdx,
			ExitIndex:     i,
			EntryPrice:    pos.entryPrice,
			ExitPrice:     price,
			InitialSize:   pos.initialSize,
			Size:          pos.size,
			RiskAmount:    pos.riskAmt,
			PnL:           pos.realized + pnl,
			Fees:          pos.fees + fee,
			R:             (pos.realized + pnl) / pos.riskAmt,
			Reason:        reason,
			PartialFilled: pos.partial,
			Strength:      pos.strength,
		})
		pos = nil
	}

	for i := warmup; i < n; i++ {
		c := candles[i]

		if pos != nil {
			barsInMarket++
			if c.High > pos.highSince {
				pos.highSince = c.High
			}
			switch {
			case c.Low <= pos.stop: // stop prima di tutto (conservativo)
				closeAt(i, pos.stop*(1-cfg.Slippage), ReasonStopLoss)
			case !pos.partial && c.High >= pos.tp1:
				pSize := pos.size * p.PartialPct
				fill := pos.tp1 * (1 - cfg.Slippage)
				fee := fill * pSize * cfg.Commission
				pnl := (fill-pos.entryPrice)*pSize - fee
				pos.realized += pnl
				realized += pnl
				pos.fees += fee
				feesTotal += fee
				pos.size -= pSize
				pos.partial = true
				if p.BreakevenOnTP1 {
					pos.stop = pos.entryPrice
				}
			case pos.tp2 > 0 && c.High >= pos.tp2:
				closeAt(i, pos.tp2*(1-cfg.Slippage), ReasonTakeProfit)
			default:
				if s.Evaluate(candles, p, i).Type == strategy.SlowExit {
					closeAt(i, c.Close*(1-cfg.Slippage), ReasonSlowExit)
				}
			}
			// trailing chandelier dopo il TP1: applicato a fine barra,
			// agisce dalle barre successive (no self-stop intrabar)
			if pos != nil && pos.partial && cfg.TrailATRMult > 0 && !math.IsNaN(s.ATR[i]) {
				trail := pos.highSince - cfg.TrailATRMult*s.ATR[i]
				if trail > pos.stop {
					pos.stop = trail
				}
			}
		}

		if pos == nil && i > lastExitIdx+cfg.CooldownBars && i > pauseUntilIdx {
			sig := s.Evaluate(candles, p, i)
			if sig.Type == strategy.BuyEntry {
				entry := c.Close * (1 + cfg.Slippage)
				riskUnit := entry - sig.StopPrice
				if riskUnit > 0 {
					eq := cfg.InitialCapital + realized
					sz := eq * cfg.RiskPct / riskUnit
					sz *= cfg.sizeFactor(sig, s.ATR[i], at(volAvg, i), peakEquity, eq)
					if cfg.MaxLeverage > 0 {
						if cap := eq * cfg.MaxLeverage / entry; sz > cap {
							sz = cap
						}
					}
					if sz > 0 {
						fee := entry * sz * cfg.Commission
						realized -= fee
						feesTotal += fee
						pos = &openPos{
							entryPrice: entry, stop: sig.StopPrice,
							tp1: sig.TP1, tp2: sig.TP2,
							size: sz, initialSize: sz,
							riskAmt:   riskUnit * sz,
							entryTime: c.Time, entryIdx: i,
							realized: -fee, fees: fee, strength: sig.Strength,
							highSince: entry,
						}
					}
				}
			}
		}

		eq := cfg.InitialCapital + realized
		if pos != nil {
			eq += (c.Close - pos.entryPrice) * pos.size
		}
		if eq > peakEquity {
			peakEquity = eq
		}
		equity = append(equity, eq)
		times = append(times, c.Time)
	}

	// chiusura forzata se resta una posizione aperta
	if pos != nil {
		i := n - 1
		closeAt(i, candles[i].Close*(1-cfg.Slippage), ReasonEndOfData)
		equity[len(equity)-1] = cfg.InitialCapital + realized
	}

	totalBars := n - warmup
	res.Trades = trades
	res.Equity = equity
	res.EquityTimes = times
	res.Metrics = ComputeMetrics(trades, equity, cfg.TimeframeMinutes, barsInMarket, totalBars, feesTotal)
	return res
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func sampleStd(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := mean(xs)
	var s float64
	for _, x := range xs {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(len(xs)-1))
}

// ComputeMetrics calcola le statistiche da trades ed equity curve.
// timeframeMinutes serve per annualizzare lo Sharpe.
func ComputeMetrics(trades []Trade, equity []float64, timeframeMinutes int,
	barsInMarket, totalBars int, feesPaid float64) Metrics {

	m := Metrics{
		BarsInMarket: barsInMarket,
		TotalBars:    totalBars,
		FeesPaid:     feesPaid,
	}
	if totalBars > 0 {
		m.Exposure = float64(barsInMarket) / float64(totalBars)
	}

	// drawdown dall'equity curve
	if len(equity) > 0 {
		peak := equity[0]
		for _, e := range equity {
			if e > peak {
				peak = e
			}
			if dd := peak - e; dd > m.MaxDrawdown {
				m.MaxDrawdown = dd
				if peak > 0 {
					m.MaxDrawdownPct = dd / peak * 100
				}
			}
		}
	}

	m.TotalTrades = len(trades)
	if m.TotalTrades == 0 {
		return m
	}

	pnls := make([]float64, len(trades))
	m.BestTrade, m.WorstTrade = math.Inf(-1), math.Inf(1)
	for i, tr := range trades {
		pnls[i] = tr.PnL
		m.NetPnL += tr.PnL
		if tr.PnL > 0 {
			m.Wins++
			m.GrossProfit += tr.PnL
		} else {
			m.GrossLoss += -tr.PnL
		}
		if tr.PnL > m.BestTrade {
			m.BestTrade = tr.PnL
		}
		if tr.PnL < m.WorstTrade {
			m.WorstTrade = tr.PnL
		}
	}

	m.WinRate = float64(m.Wins) / float64(m.TotalTrades) * 100
	m.AvgTrade = m.NetPnL / float64(m.TotalTrades)
	m.Expectancy = m.AvgTrade
	if m.GrossLoss > 0 {
		m.ProfitFactor = m.GrossProfit / m.GrossLoss
	} else {
		m.ProfitFactor = 0 // nessuna perdita: PF infinito, normalizzato a 0-report-friendly? no: uso -1? -> 0 con Wins>0 gestito dal chiamante
	}
	if m.Wins > 0 {
		m.AvgWin = m.GrossProfit / float64(m.Wins)
	}
	if losses := m.TotalTrades - m.Wins; losses > 0 {
		m.AvgLoss = m.GrossLoss / float64(losses)
	}
	if m.AvgLoss > 0 {
		m.Payoff = m.AvgWin / m.AvgLoss
	}

	std := sampleStd(pnls)
	if std > 0 {
		m.SQN = mean(pnls) / std * math.Sqrt(float64(m.TotalTrades))
	}

	// Sharpe dai rendimenti per-bar dell'equity curve, annualizzato
	if len(equity) > 2 && timeframeMinutes > 0 {
		rets := make([]float64, 0, len(equity)-1)
		for i := 1; i < len(equity); i++ {
			if equity[i-1] > 0 {
				rets = append(rets, equity[i]/equity[i-1]-1)
			}
		}
		if sd := sampleStd(rets); sd > 0 {
			barsPerYear := 525600.0 / float64(timeframeMinutes)
			m.Sharpe = mean(rets) / sd * math.Sqrt(barsPerYear)
		}
	}
	return m
}
