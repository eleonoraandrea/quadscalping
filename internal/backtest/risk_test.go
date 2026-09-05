package backtest

import (
	"math"
	"testing"

	"quadscalping/internal/market"
	"quadscalping/internal/strategy"
)

func sizeFactorOf(t *testing.T, cfg Config, cs []market.Candle) (size, sizeFactor float64) {
	t.Helper()
	res := Run("T", cs, cfg)
	if len(res.Trades) == 0 {
		t.Fatal("nessun trade")
	}
	base := DefaultConfig()
	baseRes := Run("T", cs, base)
	if len(baseRes.Trades) == 0 {
		t.Fatal("nessun trade base")
	}
	return res.Trades[0].InitialSize, res.Trades[0].InitialSize / baseRes.Trades[0].InitialSize
}

func TestSizeFactorClamps(t *testing.T) {
	sig := strategy.Signal{Strength: 100}
	cfg := DefaultConfig()
	cfg.VolAdjust = true
	// vol altissima -> fattore clampato a 0.5
	if f := cfg.sizeFactor(sig, 100, 10, 0, 0); math.Abs(f-0.5) > 1e-9 {
		t.Errorf("vol clamp want 0.5 got %v", f)
	}
	// vol bassissima -> clamp 1.25
	if f := cfg.sizeFactor(sig, 1, 10, 0, 0); math.Abs(f-1.25) > 1e-9 {
		t.Errorf("vol clamp hi want 1.25 got %v", f)
	}
	// strength 100 -> 1.0, strength 0 -> 0.5
	cfg2 := DefaultConfig()
	cfg2.StrengthSizing = true
	if f := cfg2.sizeFactor(strategy.Signal{Strength: 100}, 0, 0, 0, 0); math.Abs(f-1) > 1e-9 {
		t.Errorf("strength 100 want 1 got %v", f)
	}
	if f := cfg2.sizeFactor(strategy.Signal{Strength: 0}, 0, 0, 0, 0); math.Abs(f-0.5) > 1e-9 {
		t.Errorf("strength 0 want 0.5 got %v", f)
	}
	// dd throttle: dd pari al budget -> 0.25 (pavimento)
	cfg3 := DefaultConfig()
	cfg3.DDThrottlePct = 0.10
	if f := cfg3.sizeFactor(sig, 0, 0, 100, 90); math.Abs(f-0.25) > 1e-9 {
		t.Errorf("dd full want 0.25 got %v", f)
	}
	if f := cfg3.sizeFactor(sig, 0, 0, 100, 100); math.Abs(f-1) > 1e-9 {
		t.Errorf("no dd want 1 got %v", f)
	}
}

func TestStrengthSizingShrinksSize(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StrengthSizing = true
	cs := decline(500, 300, 0.5)
	pop(cs, 450, 1.0)
	_, ratio := sizeFactorOf(t, cfg, cs)
	// strength del segnale ~95 -> fattore ~0.975
	if ratio > 0.99 || ratio < 0.9 {
		t.Errorf("rapporto size %.3f, atteso ~0.975", ratio)
	}
}

func TestVolAdjustShrinksSizeInVolSpike(t *testing.T) {
	cfg := DefaultConfig()
	cfg.VolAdjust = true
	cs := decline(500, 300, 0.5)
	pop(cs, 450, 1.0)
	// spike di volatilità attorno all'entry: range molto più ampio
	for i := 445; i <= 450; i++ {
		cs[i].High = cs[i].Close + 6.0
		cs[i].Low = cs[i].Close - 6.0
	}
	_, ratio := sizeFactorOf(t, cfg, cs)
	if ratio >= 0.9 {
		t.Errorf("con vol spike la size deve calare, rapporto %.3f", ratio)
	}
}

func TestTrailingStopRaisesExit(t *testing.T) {
	base := DefaultConfig()
	base.Params.ExitSlow = 100 // isola il trailing dallo slow exit

	cs := decline(400, 300, 0.5)
	pop(cs, 250, 1.0)
	// rally lungo poi discesa: il trailing deve chiudere sopra l'entry
	for i := 251; i < 340; i++ {
		c := cs[i-1].Close + 1.5
		cs[i] = market.Candle{
			Time: int64(i) * 300000, Open: cs[i-1].Close,
			High: c + 0.2, Low: cs[i-1].Close - 0.1, Close: c, Volume: 100,
		}
	}
	for i := 340; i < 400; i++ {
		c := cs[i-1].Close - 1.2
		cs[i] = market.Candle{
			Time: int64(i) * 300000, Open: cs[i-1].Close,
			High: cs[i-1].Close + 0.1, Low: c - 0.2, Close: c, Volume: 100,
		}
	}

	resNoTrail := Run("T", cs, base)

	cfg := base
	cfg.TrailATRMult = 2.0
	resTrail := Run("T", cs, cfg)

	if len(resTrail.Trades) == 0 || len(resNoTrail.Trades) == 0 {
		t.Fatal("trade mancanti")
	}
	if !resTrail.Trades[0].PartialFilled {
		t.Fatalf("setup: parziale attesa (trail agisce dopo TP1)")
	}
	if resTrail.Trades[0].PnL <= resNoTrail.Trades[0].PnL {
		t.Errorf("trail deve guadagnare di più: %.2f vs %.2f",
			resTrail.Trades[0].PnL, resNoTrail.Trades[0].PnL)
	}
	if resTrail.Trades[0].ExitPrice <= resTrail.Trades[0].EntryPrice {
		t.Errorf("exit trail sopra entry: %.2f vs %.2f",
			resTrail.Trades[0].ExitPrice, resTrail.Trades[0].EntryPrice)
	}
}

func TestLossStreakPausesEntries(t *testing.T) {
	base := DefaultConfig()
	base.CooldownBars = 2 // lascia lavorare solo lo streak pause

	// serie con ripetuti pattern entry->stop
	cs := decline(1200, 3000, 0.5)
	for i := 260; i < 1200; i += 60 {
		pop(cs, i, 1.0)
	}

	resNo := Run("T", cs, base)
	cfg := base
	cfg.LossStreakN = 3
	resPause := Run("T", cs, cfg)

	if len(resNo.Trades) == 0 {
		t.Fatal("setup: trade attesi")
	}
	if len(resPause.Trades) >= len(resNo.Trades) {
		t.Errorf("pausa streak deve ridurre i trade: %d vs %d",
			len(resPause.Trades), len(resNo.Trades))
	}
}
