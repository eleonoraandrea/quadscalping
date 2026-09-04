// Package strategy implementa il rilevamento segnali HPS Quad Super Signal.
//
// Logica (fedele al sistema Python di riferimento, con correzioni):
//   - regime da EMA 20/50/200 + VWAP rolling;
//   - "quad rotation": tutti gli stochastic sotto soglia sul bar precedente;
//   - "hook": il fast %K incrocia sopra il proprio %D sul bar corrente;
//   - entry long (mean reversion) solo in regime DOWN;
//   - "slow exit": il long %K incrocia sotto il %D sopra la soglia ExitSlow.
package strategy

import (
	"math"

	"quadscalping/internal/indicator"
	"quadscalping/internal/market"
)

// SignalType identifica il tipo di segnale sulla barra valutata.
type SignalType string

const (
	None     SignalType = "NONE"
	BuyEntry SignalType = "BUY_ENTRY"
	SlowExit SignalType = "SLOW_EXIT"
)

// Regime di mercato determinato da EMA e VWAP.
type Regime string

const (
	Up   Regime = "UP"
	Down Regime = "DOWN"
	Side Regime = "SIDE"
)

// Params parametrizza la strategia HPS.
type Params struct {
	QuadFast  float64 // soglia %K fast (9) sul bar precedente
	QuadSmall float64 // soglia %K small (14)
	QuadMid   float64 // soglia %K mid (44)
	QuadLong  float64 // soglia %K long (60)

	StopMode      string  // "atr" | "swing"
	StopATR       float64 // moltiplicatore ATR sotto il low (mode atr)
	StopLookback  int     // barre per il swing low (mode swing)
	StopBufferATR float64 // buffer ATR sotto lo swing low

	TP1R           float64 // R del take profit parziale
	TP2R           float64 // R del take profit finale (0 = disabilitato)
	PartialPct     float64 // frazione chiusa al TP1
	BreakevenOnTP1 bool    // sposta lo stop a breakeven dopo il TP1

	ExitSlow    float64 // soglia %K long per lo slow exit
	MinStrength float64 // forza minima del segnale (0-100)

	VWAPPeriod   int    // periodo VWAP rolling
	RegimeFilter string // "down" = entry solo in regime DOWN, "any" = qualunque
}

// DefaultParams replica la configurazione Python di riferimento.
func DefaultParams() Params {
	return Params{
		QuadFast: 30, QuadSmall: 30, QuadMid: 30, QuadLong: 40,
		StopMode: "atr", StopATR: 1.5, StopLookback: 20, StopBufferATR: 0.1,
		TP1R: 1.5, TP2R: 0, PartialPct: 0.5, BreakevenOnTP1: true,
		ExitSlow: 70, MinStrength: 50,
		VWAPPeriod: 200, RegimeFilter: "down",
	}
}

// Series contiene tutte le serie derivate per un simbolo.
type Series struct {
	EMA20, EMA50, EMA200 []float64
	VWAP, ATR            []float64
	Kf, Df               []float64 // stochastic fast (9,3)
	Ks, Ds               []float64 // small (14,3)
	Km, Dm               []float64 // mid (44,9)
	Kl, Dl               []float64 // long (60,10)
	VolAvg20             []float64
}

// Signal è l'esito della valutazione su una barra.
type Signal struct {
	Type                            SignalType
	Strength                        float64
	Regime                          Regime
	EntryPrice, StopPrice, TP1, TP2 float64
	ATR                             float64
	Time                            int64
}

func f64s(f func(i int) float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = f(i)
	}
	return out
}

// Compute deriva tutte le serie dalle candele.
func Compute(candles []market.Candle, p Params) Series {
	n := len(candles)
	highs := f64s(func(i int) float64 { return candles[i].High }, n)
	lows := f64s(func(i int) float64 { return candles[i].Low }, n)
	closes := f64s(func(i int) float64 { return candles[i].Close }, n)
	vols := f64s(func(i int) float64 { return candles[i].Volume }, n)

	kf := indicator.StochK(highs, lows, closes, 9)
	df := indicator.StochD(kf, 3)
	ks := indicator.StochK(highs, lows, closes, 14)
	ds := indicator.StochD(ks, 3)
	km := indicator.StochK(highs, lows, closes, 44)
	dm := indicator.StochD(km, 9)
	kl := indicator.StochK(highs, lows, closes, 60)
	dl := indicator.StochD(kl, 10)

	return Series{
		EMA20:  indicator.EMA(closes, 20),
		EMA50:  indicator.EMA(closes, 50),
		EMA200: indicator.EMA(closes, 200),
		VWAP:   indicator.RollingVWAP(highs, lows, closes, vols, p.VWAPPeriod),
		ATR:    indicator.ATR(highs, lows, closes, 14),
		Kf:     kf, Df: df,
		Ks: ks, Ds: ds,
		Km: km, Dm: dm,
		Kl: kl, Dl: dl,
		VolAvg20: indicator.SMA(vols, 20),
	}
}

// WarmupBars ritorna il numero minimo di barre prima che i segnali siano validi.
func WarmupBars(p Params) int {
	m := 200 // EMA200
	if p.VWAPPeriod > m {
		m = p.VWAPPeriod
	}
	if p.StopLookback > m {
		m = p.StopLookback
	}
	return m + 10
}

func valid(xs ...float64) bool {
	for _, x := range xs {
		if math.IsNaN(x) {
			return false
		}
	}
	return true
}

// RegimeAt ritorna il regime alla barra i.
func (s *Series) RegimeAt(candles []market.Candle, i int) Regime {
	if i < 0 || i >= len(candles) || !valid(s.EMA20[i], s.EMA50[i], s.EMA200[i], s.VWAP[i]) {
		return Side
	}
	c := candles[i].Close
	if c > s.EMA200[i] && s.EMA20[i] > s.EMA50[i] && s.EMA50[i] > s.EMA200[i] && c > s.VWAP[i] {
		return Up
	}
	if c < s.EMA200[i] && s.EMA20[i] < s.EMA50[i] && s.EMA50[i] < s.EMA200[i] && c < s.VWAP[i] {
		return Down
	}
	return Side
}

// SlowExitCross verifica l'incrocio ribassista del long stochastic sopra soglia.
func SlowExitCross(k, d []float64, i int, threshold float64) bool {
	if i < 1 || i >= len(k) {
		return false
	}
	if math.IsNaN(k[i]) || math.IsNaN(d[i]) || math.IsNaN(k[i-1]) || math.IsNaN(d[i-1]) {
		return false
	}
	return k[i] < d[i] && k[i-1] >= d[i-1] && k[i] > threshold
}

// quadAt verifica la quad rotation alla barra i (barra precedente all'hook).
func (s *Series) quadAt(p Params, i int) bool {
	if i < 0 || !valid(s.Kf[i], s.Ks[i], s.Km[i], s.Kl[i]) {
		return false
	}
	return s.Kf[i] < p.QuadFast && s.Ks[i] < p.QuadSmall &&
		s.Km[i] < p.QuadMid && s.Kl[i] < p.QuadLong
}

// quadDepth misura quanto in profondità gli stochastic sono sotto soglia (0..1).
func (s *Series) quadDepth(p Params, i int) float64 {
	pairs := []struct {
		k   float64
		thr float64
	}{
		{s.Kf[i], p.QuadFast},
		{s.Ks[i], p.QuadSmall},
		{s.Km[i], p.QuadMid},
		{s.Kl[i], p.QuadLong},
	}
	var sum float64
	for _, pr := range pairs {
		if math.IsNaN(pr.k) || pr.thr <= 0 {
			continue
		}
		d := (pr.thr - pr.k) / pr.thr
		if d < 0 {
			d = 0
		}
		if d > 0.5 {
			d = 0.5
		}
		sum += d
	}
	return sum / float64(len(pairs))
}

// strength punteggio 0-100 del segnale di entry alla barra i.
func (s *Series) strength(candles []market.Candle, p Params, i int, reg Regime) float64 {
	score := 40.0 // base: quad + hook
	// il regime coerente col filtro vale il bonus pieno
	favored := Down
	if p.RegimeFilter == "up" {
		favored = Up
	}
	switch {
	case reg == favored:
		score += 25
	case reg == Side:
		score += 5
	}
	score += s.quadDepth(p, i-1) * 40 // 0..20

	margin := s.Kf[i] - s.Df[i]
	if margin > 10 {
		margin = 10
	}
	if margin > 0 {
		score += margin
	}
	if valid(s.VolAvg20[i]) && candles[i].Volume > s.VolAvg20[i] {
		score += 5
	}
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score
}

// stopPrice calcola lo stop iniziale secondo StopMode.
func (s *Series) stopPrice(candles []market.Candle, p Params, i int) float64 {
	switch p.StopMode {
	case "swing":
		lowest := math.Inf(1)
		for j := i - p.StopLookback + 1; j <= i; j++ {
			if j >= 0 && candles[j].Low < lowest {
				lowest = candles[j].Low
			}
		}
		return lowest - s.ATR[i]*p.StopBufferATR
	default: // "atr"
		return candles[i].Low - s.ATR[i]*p.StopATR
	}
}

// Evaluate valuta il segnale alla barra i (candela chiusa).
func (s *Series) Evaluate(candles []market.Candle, p Params, i int) Signal {
	n := len(candles)
	if i < 0 || i >= n || i < WarmupBars(p) {
		return Signal{Type: None}
	}
	if !valid(s.ATR[i], s.Kf[i], s.Df[i], s.Kl[i], s.Dl[i]) {
		return Signal{Type: None}
	}

	reg := s.RegimeAt(candles, i)

	// slow exit: incrocio ribassista del long stochastic sopra soglia
	if SlowExitCross(s.Kl, s.Dl, i, p.ExitSlow) {
		return Signal{Type: SlowExit, Regime: reg, Time: candles[i].Time, ATR: s.ATR[i]}
	}

	// entry: quad rotation sul bar precedente + hook sul bar corrente
	if !s.quadAt(p, i-1) {
		return Signal{Type: None}
	}
	// hook con guardia epsilon: incroci da rumore floating point non contano
	const eps = 1e-6
	hook := s.Kf[i]-s.Df[i] > eps && s.Df[i-1]-s.Kf[i-1] > -eps
	if !hook {
		return Signal{Type: None}
	}
	if p.RegimeFilter != "any" {
		required := Down
		if p.RegimeFilter == "up" {
			required = Up
		}
		if reg != required {
			return Signal{Type: None}
		}
	}

	entry := candles[i].Close
	stop := s.stopPrice(candles, p, i)
	if stop >= entry {
		return Signal{Type: None}
	}
	risk := entry - stop
	tp2 := 0.0
	if p.TP2R > 0 {
		tp2 = entry + risk*p.TP2R
	}

	st := s.strength(candles, p, i, reg)
	if st < p.MinStrength {
		return Signal{Type: None}
	}

	return Signal{
		Type:       BuyEntry,
		Strength:   st,
		Regime:     reg,
		EntryPrice: entry,
		StopPrice:  stop,
		TP1:        entry + risk*p.TP1R,
		TP2:        tp2,
		ATR:        s.ATR[i],
		Time:       candles[i].Time,
	}
}

// EvaluateLast valuta l'ultima barra chiusa.
func (s *Series) EvaluateLast(candles []market.Candle, p Params) Signal {
	if len(candles) == 0 {
		return Signal{Type: None}
	}
	return s.Evaluate(candles, p, len(candles)-1)
}
