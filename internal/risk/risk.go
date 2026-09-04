// Package risk gestisce position sizing e limiti di rischio giornalieri.
package risk

import "time"

// Manager applica le regole di rischio del bot.
type Manager struct {
	RiskPct         float64 // rischio per trade (frazione dell'equity)
	MaxDailyLossPct float64 // stop giornaliero (frazione dell'equity iniziale giornata)
	MaxPositions    int

	day         int // yyyymmdd del giorno corrente
	startOfDay  float64
	dailyPnL    float64
	dailyTrades int
}

// New crea il manager.
func New(riskPct, maxDailyLossPct float64, maxPositions int) *Manager {
	return &Manager{
		RiskPct:         riskPct,
		MaxDailyLossPct: maxDailyLossPct,
		MaxPositions:    maxPositions,
		day:             0,
	}
}

func dayOf(t time.Time) int {
	y, m, d := t.Date()
	return y*10000 + int(m)*100 + d
}

// rollDay resetta le statistiche giornaliere al cambio giorno.
func (m *Manager) rollDay(now time.Time) {
	d := dayOf(now)
	if d != m.day {
		m.day = d
		m.startOfDay = 0 // impostato al primo RecordEquity/canOpen del giorno
		m.dailyPnL = 0
		m.dailyTrades = 0
	}
}

// RecordPnL registra un pnl realizzato (con roll giornaliero).
func (m *Manager) RecordPnL(now time.Time, equity, pnl float64) {
	m.rollDay(now)
	if m.startOfDay == 0 {
		m.startOfDay = equity
	}
	m.dailyPnL += pnl
	m.dailyTrades++
}

// DailyPnL ritorna il pnl del giorno corrente.
func (m *Manager) DailyPnL() float64 { return m.dailyPnL }

// DailyTrades ritorna i trade del giorno corrente.
func (m *Manager) DailyTrades() int { return m.dailyTrades }

// CanOpen verifica i limiti prima di aprire una posizione.
func (m *Manager) CanOpen(now time.Time, equity float64, openPositions int) (bool, string) {
	m.rollDay(now)
	if m.startOfDay == 0 {
		m.startOfDay = equity
	}
	if openPositions >= m.MaxPositions {
		return false, "raggiunto numero massimo posizioni"
	}
	if m.MaxDailyLossPct > 0 && m.startOfDay > 0 &&
		m.dailyPnL <= -m.startOfDay*m.MaxDailyLossPct {
		return false, "limite perdita giornaliera raggiunto"
	}
	return true, ""
}

// PositionSize calcola la size per rischiare RiskPct dell'equity tra entry e stop.
func (m *Manager) PositionSize(equity, entry, stop float64) float64 {
	riskUnit := entry - stop
	if riskUnit <= 0 || equity <= 0 {
		return 0
	}
	return equity * m.RiskPct / riskUnit
}
