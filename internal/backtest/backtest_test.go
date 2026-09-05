package backtest

import (
	"math"
	"testing"

	"quadscalping/internal/market"
	"quadscalping/internal/strategy"
)

// decline: candele in discesa lineare, range costante (come strategy_test).
func decline(n int, start, step float64) []market.Candle {
	cs := make([]market.Candle, n)
	for i := 0; i < n; i++ {
		c := start - step*float64(i)
		cs[i] = market.Candle{
			Time: int64(i) * 300000, Open: c + step/2,
			High: c + 0.2, Low: c - 0.2, Close: c, Volume: 100,
		}
	}
	return cs
}

// pop solleva la chiusura della barra i di amount.
func pop(cs []market.Candle, i int, amount float64) {
	c := cs[i].Close + amount
	cs[i].Close = c
	cs[i].High = c + 0.2
	cs[i].Open = c
}

func series(cs []market.Candle) *strategy.Series {
	s := strategy.Compute(cs, strategy.DefaultParams())
	return &s
}

func TestStopLossScenario(t *testing.T) {
	cfg := DefaultConfig()
	cs := decline(500, 300, 0.5)
	pop(cs, 450, 1.0) // entry alla barra 450, poi la discesa prosegue

	res := Run("TESTUSDT", cs, cfg)
	if len(res.Trades) != 1 {
		t.Fatalf("trades=%d want 1", len(res.Trades))
	}
	tr := res.Trades[0]
	if tr.Reason != ReasonStopLoss {
		t.Fatalf("reason=%v want STOP_LOSS", tr.Reason)
	}
	if tr.EntryIndex != 450 {
		t.Fatalf("entryIndex=%d want 450", tr.EntryIndex)
	}

	// replica la matematica attesa
	s := series(cs)
	i := 450
	entry := cs[i].Close * (1 + cfg.Slippage)
	stop := cs[i].Low - s.ATR[i]*cfg.Params.StopATR
	exit := stop * (1 - cfg.Slippage)
	riskUnit := entry - stop
	size := cfg.InitialCapital * cfg.RiskPct / riskUnit
	fees := entry*size*cfg.Commission + exit*size*cfg.Commission
	wantPnL := (exit-entry)*size - fees

	if math.Abs(tr.EntryPrice-entry) > 1e-9 {
		t.Errorf("entry %v want %v", tr.EntryPrice, entry)
	}
	if math.Abs(tr.ExitPrice-exit) > 1e-9 {
		t.Errorf("exit %v want %v", tr.ExitPrice, exit)
	}
	if math.Abs(tr.Size-size) > 1e-9 {
		t.Errorf("size %v want %v", tr.Size, size)
	}
	if math.Abs(tr.PnL-wantPnL) > 1e-6 {
		t.Errorf("pnl %v want %v", tr.PnL, wantPnL)
	}
	if tr.PnL >= 0 {
		t.Errorf("stop loss deve essere negativo: %v", tr.PnL)
	}
	if res.Metrics.NetPnL > 0 {
		t.Errorf("net pnl su stop deve essere <= 0: %v", res.Metrics.NetPnL)
	}
}

func TestLeverageCapOnTightStop(t *testing.T) {
	cfg := DefaultConfig()
	// prezzi alti: stop stretto (risk unit ~1.2) ma entry ~29775
	// -> sizing non cappato would be ~80 unita' (notionale ~2.4M) >> equity
	cs := decline(500, 30000, 0.5)
	pop(cs, 450, 1.0)
	cs[450].Low = cs[450].Close - 0.05

	res := Run("TESTUSDT", cs, cfg)
	if len(res.Trades) != 1 {
		t.Fatalf("trades=%d", len(res.Trades))
	}
	tr := res.Trades[0]
	s := series(cs)
	i := 450
	entry := cs[i].Close * (1 + cfg.Slippage)
	stop := cs[i].Low - s.ATR[i]*cfg.Params.StopATR
	uncapped := cfg.InitialCapital * cfg.RiskPct / (entry - stop)
	capped := cfg.InitialCapital * cfg.MaxLeverage / entry
	if uncapped <= capped {
		t.Fatalf("test mal configurato: uncapped %v <= capped %v", uncapped, capped)
	}
	if math.Abs(tr.Size-capped) > 1e-9 {
		t.Errorf("size %v want cap %v", tr.Size, capped)
	}
}

func TestPartialTPAndSlowExit(t *testing.T) {
	cfg := DefaultConfig()
	cs := decline(400, 300, 0.5)
	pop(cs, 250, 1.0) // entry

	// poi rally lungo: 150 barre in salita (il long stochastic sale sopra 70
	// e prima o dopo incrocia sotto -> SLOW_EXIT)
	for i := 251; i < 400; i++ {
		c := cs[i-1].Close + 1.5
		cs[i] = market.Candle{
			Time: int64(i) * 300000, Open: cs[i-1].Close,
			High: c + 0.2, Low: cs[i-1].Close - 0.1, Close: c, Volume: 100,
		}
	}

	res := Run("TESTUSDT", cs, cfg)
	if len(res.Trades) != 1 {
		t.Fatalf("trades=%d want 1", len(res.Trades))
	}
	tr := res.Trades[0]
	if !tr.PartialFilled {
		t.Errorf("want partial filled")
	}
	if tr.PnL <= 0 {
		t.Errorf("rally dopo entry: pnl deve essere positivo, got %v", tr.PnL)
	}
	if tr.Reason != ReasonSlowExit && tr.Reason != ReasonEndOfData {
		t.Errorf("reason %v", tr.Reason)
	}
	if tr.Size >= tr.InitialSize {
		t.Errorf("size dopo parziale deve scendere: %v >= %v", tr.Size, tr.InitialSize)
	}
}

func TestEndOfDataClose(t *testing.T) {
	cfg := DefaultConfig()
	cs := decline(400, 300, 0.5)
	pop(cs, 399, 1.0) // entry sull'ultima barra -> chiusura forzata

	res := Run("TESTUSDT", cs, cfg)
	if len(res.Trades) != 1 {
		t.Fatalf("trades=%d want 1", len(res.Trades))
	}
	if res.Trades[0].Reason != ReasonEndOfData {
		t.Errorf("reason %v want END_OF_DATA", res.Trades[0].Reason)
	}
}

func TestEquityCurveLengthAndStart(t *testing.T) {
	cfg := DefaultConfig()
	cs := decline(300, 300, 0.5)
	res := Run("TESTUSDT", cs, cfg)
	if len(res.Equity) == 0 {
		t.Fatal("equity vuota")
	}
	if math.Abs(res.Equity[0]-cfg.InitialCapital) > 1e-6 {
		t.Errorf("equity[0]=%v want %v", res.Equity[0], cfg.InitialCapital)
	}
	if len(res.Equity) != len(res.EquityTimes) {
		t.Errorf("equity/times mismatch")
	}
}

func TestComputeMetrics(t *testing.T) {
	trades := []Trade{
		{PnL: 100}, {PnL: -50}, {PnL: 200}, {PnL: -50},
	}
	equity := []float64{10000, 10100, 10050, 10250, 10200}
	m := ComputeMetrics(trades, equity, 5, 60, 200, 12.5)

	if m.TotalTrades != 4 || m.Wins != 2 {
		t.Errorf("total=%d wins=%d", m.TotalTrades, m.Wins)
	}
	if math.Abs(m.WinRate-50) > 1e-9 {
		t.Errorf("winrate %v", m.WinRate)
	}
	if math.Abs(m.ProfitFactor-3) > 1e-9 {
		t.Errorf("PF %v want 3", m.ProfitFactor)
	}
	if math.Abs(m.NetPnL-200) > 1e-9 {
		t.Errorf("netpnl %v", m.NetPnL)
	}
	if math.Abs(m.AvgTrade-50) > 1e-9 {
		t.Errorf("avg %v", m.AvgTrade)
	}
	// equity: 10000 -> 10100 -> 10050 -> 10250 -> 10200 ; picchi 10100, 10250
	if math.Abs(m.MaxDrawdown-50) > 1e-9 {
		t.Errorf("maxdd %v want 50", m.MaxDrawdown)
	}
	if math.Abs(m.MaxDrawdownPct-50.0/10100.0*100) > 1e-9 {
		t.Errorf("maxddpct %v", m.MaxDrawdownPct)
	}
	if math.Abs(m.FeesPaid-12.5) > 1e-9 {
		t.Errorf("fees %v", m.FeesPaid)
	}
	if math.Abs(m.Exposure-0.3) > 1e-9 {
		t.Errorf("exposure %v", m.Exposure)
	}
	if m.SQN <= 0 {
		t.Errorf("sqn %v", m.SQN)
	}
}

func TestComputeMetricsNoTrades(t *testing.T) {
	m := ComputeMetrics(nil, []float64{10000, 10000, 10000}, 5, 0, 3, 0)
	if m.TotalTrades != 0 || m.ProfitFactor != 0 || m.WinRate != 0 {
		t.Errorf("metriche vuote sbagliate: %+v", m)
	}
	if m.MaxDrawdown != 0 {
		t.Errorf("maxdd %v", m.MaxDrawdown)
	}
}
