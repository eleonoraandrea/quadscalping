package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"quadscalping/internal/backtest"
	"quadscalping/internal/market"
)

func sampleResult() backtest.Result {
	cfg := backtest.DefaultConfig()
	cs := make([]market.Candle, 0, 320)
	base := 300.0
	for i := 0; i < 320; i++ {
		c := base - 0.4*float64(i)
		cs = append(cs, market.Candle{
			Time: int64(1700000000000 + i*300000),
			Open: c + 0.2, High: c + 0.3, Low: c - 0.3, Close: c, Volume: 100,
		})
	}
	// pop per innescare un entry
	c := cs[300].Close + 1.2
	cs[300].Close = c
	cs[300].High = c + 0.2
	res := backtest.Run("TESTUSDT", cs, cfg)
	return res
}

func TestGenerateHTMLContainsCoreSections(t *testing.T) {
	res := sampleResult()
	d := Data{
		Title:       "Report Test",
		GeneratedAt: time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC),
		Symbols: []SymbolReport{{
			Symbol: "TESTUSDT", Timeframe: "5m",
			From: time.UnixMilli(1700000000000), To: time.UnixMilli(1700096000000),
			Result: res,
		}},
	}
	html, err := GenerateHTML(d)
	if err != nil {
		t.Fatal(err)
	}
	checks := []string{
		"<svg", "TESTUSDT", "5m", "Equity", "Drawdown",
		"Win rate", "Profit factor", "PnL", "STOP_LOSS",
		"prefers-color-scheme", "tabular-nums",
	}
	for _, want := range checks {
		if !strings.Contains(html, want) {
			t.Errorf("HTML senza %q", want)
		}
	}
	// tabella trade: almeno il primo trade presente
	if len(res.Trades) > 0 {
		if !strings.Contains(html, "STOP_LOSS") {
			t.Error("motivo uscita mancante in tabella")
		}
	}
	// no dipendenze esterne
	if strings.Contains(html, "http://") || strings.Contains(html, "https://") {
		t.Error("report deve essere self-contained (nessun URL esterno)")
	}
}

func TestWriteFile(t *testing.T) {
	res := sampleResult()
	path := filepath.Join(t.TempDir(), "report.html")
	err := WriteFile(path, Data{
		Title: "T", GeneratedAt: time.Now(),
		Symbols: []SymbolReport{{Symbol: "X", Timeframe: "5m", Result: res}},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "<!DOCTYPE html>") {
		t.Error("doctype mancante")
	}
}

func TestNiceTicks(t *testing.T) {
	ticks := NiceTicks(0, 100, 5)
	if len(ticks) < 3 {
		t.Fatalf("troppi pochi tick: %v", ticks)
	}
	for i := 1; i < len(ticks); i++ {
		if ticks[i] <= ticks[i-1] {
			t.Errorf("tick non monotoni: %v", ticks)
		}
	}
	if ticks[0] > 0 || ticks[len(ticks)-1] < 100 {
		t.Errorf("range non coperto: %v", ticks)
	}
	// step pulito
	step := ticks[1] - ticks[0]
	for i := 1; i < len(ticks); i++ {
		if ticks[i]-ticks[i-1]-step > 1e-9 {
			t.Errorf("step non costante: %v", ticks)
		}
	}
}

func TestNiceTicksDegenerate(t *testing.T) {
	ticks := NiceTicks(5, 5, 5)
	if len(ticks) < 2 {
		t.Errorf("range degenere deve comunque dare tick: %v", ticks)
	}
}

func TestDownsample(t *testing.T) {
	xs := make([]float64, 10000)
	for i := range xs {
		xs[i] = float64(i)
	}
	out := Downsample(xs, 500)
	if len(out) != 500 {
		t.Fatalf("len %d want 500", len(out))
	}
	if out[0] != 0 || out[len(out)-1] != 9999 {
		t.Errorf("estremi non preservati: %v..%v", out[0], out[len(out)-1])
	}
	small := Downsample([]float64{1, 2, 3}, 500)
	if len(small) != 3 {
		t.Errorf("serie corta non va toccata: %v", small)
	}
}
