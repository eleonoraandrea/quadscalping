package indicator

import (
	"math"
	"testing"
)

func line(n int, start, step float64) []float64 {
	xs := make([]float64, n)
	for i := range xs {
		xs[i] = start + step*float64(i)
	}
	return xs
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSMA(t *testing.T) {
	got := SMA(line(10, 1, 1), 3)
	if len(got) != 10 {
		t.Fatalf("len=%d want 10", len(got))
	}
	if !math.IsNaN(got[1]) {
		t.Errorf("warmup[1] not NaN: %v", got[1])
	}
	if !approx(got[2], 2) || !approx(got[3], 3) || !approx(got[9], 9) {
		t.Errorf("SMA values wrong: %v", got)
	}
}

func TestSMAWithLeadingNaN(t *testing.T) {
	// serie con NaN di warmup in testa (es. ATR): la SMA non deve
	// restare avvelenata per sempre
	xs := []float64{math.NaN(), math.NaN(), 1, 2, 3, 4, 5}
	got := SMA(xs, 3)
	if !math.IsNaN(got[3]) {
		t.Errorf("warmup relativo non NaN: %v", got[3])
	}
	if !approx(got[4], 2) || !approx(got[5], 3) || !approx(got[6], 4) {
		t.Errorf("SMA con NaN in testa sbagliata: %v", got)
	}
}

func TestEMA(t *testing.T) {
	got := EMA(line(10, 1, 1), 3)
	if !math.IsNaN(got[1]) {
		t.Errorf("warmup not NaN")
	}
	// seed = SMA(3) = 2, alpha = 2/(3+1) = 0.5
	// idx3: 0.5*4+0.5*2 = 3 ; idx4: 0.5*5+0.5*3 = 4 ...
	if !approx(got[2], 2) || !approx(got[3], 3) || !approx(got[4], 4) || !approx(got[9], 9) {
		t.Errorf("EMA values wrong: %v", got)
	}
}

func TestStochK(t *testing.T) {
	closes := []float64{10, 11, 12}
	highs := []float64{10.5, 11.5, 12.5}
	lows := []float64{9.5, 10.5, 11.5}
	got := StochK(highs, lows, closes, 3)
	if !math.IsNaN(got[1]) {
		t.Errorf("warmup not NaN")
	}
	// HH=12.5 LL=9.5 C=12 -> 100*(12-9.5)/3 = 83.333...
	if !approx(got[2], 100*(12-9.5)/(12.5-9.5)) {
		t.Errorf("StochK wrong: %v", got[2])
	}
}

func TestStochKFlatWindowIsNeutral(t *testing.T) {
	c := []float64{5, 5, 5}
	got := StochK(c, c, c, 3)
	if !approx(got[2], 50) {
		t.Errorf("flat window want 50, got %v", got[2])
	}
}

func TestStochD(t *testing.T) {
	k := []float64{math.NaN(), 20, 40, 60}
	got := StochD(k, 2)
	if !approx(got[2], 30) || !approx(got[3], 50) {
		t.Errorf("StochD wrong: %v", got)
	}
}

func TestATR(t *testing.T) {
	// costanti: H=2 L=1 C=1.5 -> TR = 1 sempre
	n := 5
	c := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	for i := 0; i < n; i++ {
		h[i], l[i], c[i] = 2, 1, 1.5
	}
	got := ATR(h, l, c, 2)
	// TR valido da idx1; ATR(2) valido da idx2
	if !math.IsNaN(got[1]) || !approx(got[2], 1) || !approx(got[4], 1) {
		t.Errorf("ATR wrong: %v", got)
	}
}

func TestRollingVWAP(t *testing.T) {
	h := []float64{1.5, 2.5}
	l := []float64{0.5, 1.5}
	c := []float64{1, 2}
	v := []float64{1, 3}
	got := RollingVWAP(h, l, c, v, 2)
	// TP: 1, 2 -> (1*1 + 2*3)/(1+3) = 1.75
	if !approx(got[1], 1.75) {
		t.Errorf("VWAP want 1.75 got %v", got[1])
	}
}

func TestRollingVWAPZeroVolumeFallsBackToSMA(t *testing.T) {
	h := []float64{1.5, 2.5}
	l := []float64{0.5, 1.5}
	c := []float64{1, 2}
	v := []float64{0, 0}
	got := RollingVWAP(h, l, c, v, 2)
	if !approx(got[1], 1.5) {
		t.Errorf("VWAP fallback want 1.5 got %v", got[1])
	}
}
