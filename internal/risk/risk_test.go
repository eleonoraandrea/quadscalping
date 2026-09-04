package risk

import (
	"math"
	"testing"
	"time"
)

var t0 = time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)

func TestPositionSize(t *testing.T) {
	m := New(0.01, 0.05, 3)
	size := m.PositionSize(10000, 100, 98)
	if math.Abs(size-50) > 1e-9 { // 100 / 2 = 50
		t.Errorf("size %v want 50", size)
	}
	if m.PositionSize(10000, 100, 101) != 0 {
		t.Error("stop sopra entry deve dare 0")
	}
	if m.PositionSize(0, 100, 98) != 0 {
		t.Error("equity 0 deve dare 0")
	}
}

func TestCanOpenMaxPositions(t *testing.T) {
	m := New(0.01, 0.05, 2)
	if ok, _ := m.CanOpen(t0, 10000, 1); !ok {
		t.Error("1/2 posizioni deve essere ok")
	}
	if ok, reason := m.CanOpen(t0, 10000, 2); ok {
		t.Error("3/2 posizioni deve bloccare")
	} else if reason == "" {
		t.Error("motivo mancante")
	}
}

func TestDailyLossLimit(t *testing.T) {
	m := New(0.01, 0.05, 3)
	// perdi 400 su equity 10000: sotto il 5% (500) ancora ok
	m.RecordPnL(t0, 10000, -400)
	if ok, _ := m.CanOpen(t0.Add(time.Hour), 9600, 0); !ok {
		t.Error("-400 su 10000 non deve bloccare (limite 500)")
	}
	// altra perdita: -101 -> totale -501 sotto -500
	m.RecordPnL(t0.Add(2*time.Hour), 9600, -101)
	if ok, reason := m.CanOpen(t0.Add(3*time.Hour), 9500, 0); ok {
		t.Error("perdita giornaliera superata deve bloccare")
	} else if reason == "" {
		t.Error("motivo mancante")
	}
}

func TestDailyRollResets(t *testing.T) {
	m := New(0.01, 0.05, 3)
	m.RecordPnL(t0, 10000, -600)
	if ok, _ := m.CanOpen(t0.Add(3*time.Hour), 9400, 0); ok {
		t.Fatal("dovrebbe essere bloccato")
	}
	nextDay := t0.Add(24 * time.Hour)
	if ok, _ := m.CanOpen(nextDay, 9400, 0); !ok {
		t.Error("giorno nuovo: limiti resettati")
	}
	if m.DailyPnL() != 0 || m.DailyTrades() != 0 {
		t.Errorf("statistiche non resettate: %v %v", m.DailyPnL(), m.DailyTrades())
	}
}
