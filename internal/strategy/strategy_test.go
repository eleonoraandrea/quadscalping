package strategy

import (
	"math"
	"testing"

	"quadscalping/internal/market"
)

// pop solleva la chiusura della barra i di amount.
func pop(cs []market.Candle, i int, amount float64) {
	c := cs[i].Close + amount
	cs[i].Close = c
	cs[i].High = c + 0.2
	cs[i].Open = c
}

// decline genera n candele in discesa lineare con range costante.
func decline(n int, start, step float64) []market.Candle {
	cs := make([]market.Candle, n)
	for i := 0; i < n; i++ {
		c := start - step*float64(i)
		cs[i] = market.Candle{
			Time:   int64(i) * 300000,
			Open:   c + step/2,
			High:   c + 0.2,
			Low:    c - 0.2,
			Close:  c,
			Volume: 100,
		}
	}
	return cs
}

func TestRegimeDownOnDecline(t *testing.T) {
	p := DefaultParams()
	cs := decline(400, 300, 0.5)
	s := Compute(cs, p)
	if got := s.RegimeAt(cs, len(cs)-1); got != Down {
		t.Errorf("regime want DOWN got %v", got)
	}
}

func TestRegimeUpOnRally(t *testing.T) {
	p := DefaultParams()
	// rally lineare: invertiamo il passo
	cs := decline(400, 100, -0.5)
	s := Compute(cs, p)
	if got := s.RegimeAt(cs, len(cs)-1); got != Up {
		t.Errorf("regime want UP got %v", got)
	}
}

func TestBuyEntryOnQuadHook(t *testing.T) {
	p := DefaultParams()
	cs := decline(400, 300, 0.5)
	// pop sull'ultima candela: hook del fast stochastic
	last := len(cs) - 1
	pop := cs[last].Close + 1.0
	cs[last].Close = pop
	cs[last].High = pop + 0.2
	cs[last].Open = pop

	s := Compute(cs, p)
	sig := s.Evaluate(cs, p, last)

	if sig.Type != BuyEntry {
		t.Fatalf("want BUY_ENTRY got %v (regime=%v)", sig.Type, sig.Regime)
	}
	if sig.Strength < p.MinStrength || sig.Strength > 100 {
		t.Errorf("strength out of range: %v", sig.Strength)
	}
	if sig.EntryPrice != cs[last].Close {
		t.Errorf("entry %v want %v", sig.EntryPrice, cs[last].Close)
	}
	wantStop := cs[last].Low - sig.ATR*p.StopATR
	if math.Abs(sig.StopPrice-wantStop) > 1e-9 {
		t.Errorf("stop %v want %v", sig.StopPrice, wantStop)
	}
	risk := sig.EntryPrice - sig.StopPrice
	if math.Abs(sig.TP1-(sig.EntryPrice+risk*p.TP1R)) > 1e-9 {
		t.Errorf("tp1 %v", sig.TP1)
	}
	if sig.StopPrice >= cs[last].Low {
		t.Errorf("stop deve stare sotto il low: %v >= %v", sig.StopPrice, cs[last].Low)
	}
}

func TestNoEntryWithoutHook(t *testing.T) {
	p := DefaultParams()
	cs := decline(400, 300, 0.5) // nessun pop: nessun hook
	s := Compute(cs, p)
	sig := s.Evaluate(cs, p, len(cs)-1)
	if sig.Type != None {
		t.Errorf("want NONE got %v", sig.Type)
	}
}

func TestNoEntryInUptrendWithDownFilter(t *testing.T) {
	p := DefaultParams()
	cs := decline(400, 100, -0.5) // rally
	last := len(cs) - 1
	pop := cs[last].Close + 1.0
	cs[last].Close = pop
	cs[last].High = pop + 0.2
	s := Compute(cs, p)
	sig := s.Evaluate(cs, p, last)
	if sig.Type != None {
		t.Errorf("regime UP con filtro down: want NONE got %v", sig.Type)
	}
}

func TestEvaluateBeforeWarmupIsNone(t *testing.T) {
	p := DefaultParams()
	cs := decline(300, 300, 0.5)
	s := Compute(cs, p)
	if got := s.Evaluate(cs, p, 10); got.Type != None {
		t.Errorf("before warmup want NONE got %v", got.Type)
	}
	if WarmupBars(p) < 200 {
		t.Errorf("warmup troppo corto: %d", WarmupBars(p))
	}
}

// manualSeries costruisce una Series sintetica con quad+hook alla fine
// e regime dato (via EMA/VWAP), per testare la sola logica dei filtri.
func manualSeries(n int, reg Regime) (Series, []market.Candle) {
	const kLow = 5.0 // sotto Df: il cross e' possibile
	const dLow = 8.0
	s := Series{
		Kf: make([]float64, n), Df: make([]float64, n),
		Ks: make([]float64, n), Ds: make([]float64, n),
		Km: make([]float64, n), Dm: make([]float64, n),
		Kl: make([]float64, n), Dl: make([]float64, n),
	}
	for i := 0; i < n; i++ {
		s.Ks[i], s.Ds[i], s.Km[i], s.Dm[i], s.Kl[i], s.Dl[i] = kLow, kLow, kLow, kLow, kLow, kLow
		s.Kf[i], s.Df[i] = kLow, dLow
	}
	last := n - 1
	s.Kf[last] = 40 // hook: Kf incrocia sopra Df
	s.Kf[last-1] = kLow
	s.Df[last-1] = dLow
	s.Df[last] = dLow

	// candele piatte a 100 con EMA/VWAP coerenti col regime
	candles := make([]market.Candle, n)
	for i := range candles {
		candles[i] = market.Candle{
			Time: int64(i) * 300000, Open: 100, High: 100.2,
			Low: 99.8, Close: 100, Volume: 100,
		}
	}
	ema := func(v float64) []float64 {
		out := make([]float64, n)
		for i := range out {
			out[i] = v
		}
		return out
	}
	s.ATR = ema(1.0)
	s.VolAvg20 = ema(100)
	switch reg {
	case Up:
		s.EMA20, s.EMA50, s.EMA200 = ema(99), ema(98), ema(97) // 100>99>98>97
		s.VWAP = ema(99)                                       // close > vwap
	case Down:
		s.EMA20, s.EMA50, s.EMA200 = ema(101), ema(102), ema(103)
		s.VWAP = ema(101) // close < vwap
	default:
		s.EMA20, s.EMA50, s.EMA200 = ema(101), ema(102), ema(97) // misto
		s.VWAP = ema(99)
	}
	return s, candles
}

func TestRegimeFilterMatrix(t *testing.T) {
	cases := []struct {
		regime Regime
		filter string
		want   SignalType
	}{
		{Down, "down", BuyEntry},
		{Down, "up", None},
		{Up, "up", BuyEntry},
		{Up, "down", None},
		{Side, "any", BuyEntry},
		{Side, "down", None},
		{Side, "up", None},
	}
	for _, c := range cases {
		s, cs := manualSeries(250, c.regime)
		p := DefaultParams()
		p.RegimeFilter = c.filter
		got := s.Evaluate(cs, p, len(cs)-1).Type
		if got != c.want {
			t.Errorf("regime=%v filter=%s: got %v want %v", c.regime, c.filter, got, c.want)
		}
	}
}

func TestSlowExitCross(t *testing.T) {
	k := []float64{60, 85, 75}
	d := []float64{60, 70, 78}
	if !SlowExitCross(k, d, 2, 70) {
		t.Errorf("cross valido non rilevato")
	}
	// sotto soglia: niente uscita
	k2 := []float64{60, 75, 65}
	d2 := []float64{60, 70, 72}
	if SlowExitCross(k2, d2, 2, 70) {
		t.Errorf("cross sotto soglia non deve uscire")
	}
	// nessun incrocio
	k3 := []float64{60, 80, 78}
	d3 := []float64{60, 70, 71}
	if SlowExitCross(k3, d3, 2, 70) {
		t.Errorf("nessun incrocio non deve uscire")
	}
}

func TestSwingStopMode(t *testing.T) {
	p := DefaultParams()
	p.StopMode = "swing"
	p.StopLookback = 20
	p.StopBufferATR = 0.5
	cs := decline(400, 300, 0.5)
	last := len(cs) - 1
	pop := cs[last].Close + 1.0
	cs[last].Close = pop
	cs[last].High = pop + 0.2

	s := Compute(cs, p)
	sig := s.Evaluate(cs, p, last)
	if sig.Type != BuyEntry {
		t.Fatalf("want BUY_ENTRY got %v", sig.Type)
	}
	// swing low = min low ultimi 20 bar (il low pi� vecchio, discesa)
	lowest := math.Inf(1)
	for j := last - p.StopLookback + 1; j <= last; j++ {
		if cs[j].Low < lowest {
			lowest = cs[j].Low
		}
	}
	want := lowest - sig.ATR*p.StopBufferATR
	if math.Abs(sig.StopPrice-want) > 1e-9 {
		t.Errorf("swing stop %v want %v", sig.StopPrice, want)
	}
}
