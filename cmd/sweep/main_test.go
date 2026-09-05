package main

import (
	"testing"

	"quadscalping/internal/market"
)

func TestGrid(t *testing.T) {
	g := Grid([]string{"1h"}, []float64{1.5, 2}, []float64{1}, []float64{0}, []float64{70}, []string{"down"}, []float64{50, 70})
	if len(g) != 4 {
		t.Fatalf("len %d want 4", len(g))
	}
	if g[0].Interval != "1h" || g[0].StopATR != 1.5 || g[0].MinStrength != 50 {
		t.Errorf("combo 0 sbagliata: %+v", g[0])
	}
	if g[3].MinStrength != 70 {
		t.Errorf("combo 3 sbagliata: %+v", g[3])
	}
}

func TestSplitIS(t *testing.T) {
	cs := make([]market.Candle, 100)
	for i := range cs {
		cs[i] = market.Candle{Time: int64(i)}
	}
	is, oos := SplitIS(cs, 0.7)
	if len(is) != 70 || len(oos) != 30 {
		t.Fatalf("70/30 -> %d/%d", len(is), len(oos))
	}
	if is[69].Time != 69 || oos[0].Time != 70 {
		t.Errorf("contiguità rotta")
	}
}
