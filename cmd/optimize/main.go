package main

import (
"fmt"
"log"
"math"
"quadscalping/internal/backtest"
"quadscalping/internal/data"
"quadscalping/internal/strategy"
)

func main() {
symbols := []string{"BTCUSDT", "ETHUSDT", "DOGEUSDT"}
tf := "4h"

bestOverall := make(map[string]backtest.Result)
bestOverallScore := make(map[string]float64)

for _, sym := range symbols {
fmt.Printf("\n=== %s ===\n", sym)
path := fmt.Sprintf("data/%s_%s.csv", sym, tf)
cs, err := data.LoadCSV(path)
if err != nil {
log.Fatalf("Errore load %s: %v", sym, err)
}
fmt.Printf("Caricate %d candele\n", len(cs))

var bestRes backtest.Result
bestScore := math.Inf(-1)
bestName := ""

testMatrix := []struct {
name        string
stopATR     float64
tp1R        float64
minStrength float64
exitSlow    float64
trailATR    float64
regime      string
}{
{"base", 1.5, 1.5, 50, 70, 0, "down"},
{"stop_1.0", 1.0, 1.5, 50, 70, 0, "down"},
{"stop_2.0", 2.0, 1.5, 50, 70, 0, "down"},
{"stop_2.5", 2.5, 1.5, 50, 70, 0, "down"},
{"stop_3.0", 3.0, 1.5, 50, 70, 0, "down"},
{"tp_2.0", 1.5, 2.0, 50, 70, 0, "down"},
{"tp_2.5", 1.5, 2.5, 50, 70, 0, "down"},
{"tp_3.0", 1.5, 3.0, 50, 70, 0, "down"},
{"tp_4.0", 1.5, 4.0, 50, 70, 0, "down"},
{"tp_5.0", 1.5, 5.0, 50, 70, 0, "down"},
{"str_60", 1.5, 1.5, 60, 70, 0, "down"},
{"str_70", 1.5, 1.5, 70, 70, 0, "down"},
{"str_80", 1.5, 1.5, 80, 70, 0, "down"},
{"exit_60", 1.5, 1.5, 50, 60, 0, "down"},
{"exit_65", 1.5, 1.5, 50, 65, 0, "down"},
{"exit_75", 1.5, 1.5, 50, 75, 0, "down"},
{"exit_80", 1.5, 1.5, 50, 80, 0, "down"},
{"trail_1.0", 1.5, 1.5, 50, 70, 1.0, "down"},
{"trail_1.5", 1.5, 1.5, 50, 70, 1.5, "down"},
{"trail_2.0", 1.5, 1.5, 50, 70, 2.0, "down"},
{"any_regime", 1.5, 1.5, 50, 70, 0, "any"},
{"str70_tp3", 1.5, 3.0, 70, 70, 0, "down"},
{"str60_exit65", 1.5, 1.5, 60, 65, 0, "down"},
{"stop2_tp3", 2.0, 3.0, 50, 70, 0, "down"},
{"trail1.5_tp2.5", 1.5, 2.5, 50, 70, 1.5, "down"},
{"str70_trail1.5", 1.5, 1.5, 70, 70, 1.5, "down"},
}

for _, tc := range testMatrix {
cfg := backtest.DefaultConfig()
cfg.InitialCapital = 10000
cfg.RiskPct = 0.01
cfg.Params = strategy.Params{
QuadFast: 30, QuadSmall: 30, QuadMid: 30, QuadLong: 40,
StopMode: "atr", StopATR: tc.stopATR, StopLookback: 20, StopBufferATR: 0.1,
TP1R: tc.tp1R, TP2R: 0, PartialPct: 0.5, BreakevenOnTP1: true,
ExitSlow: tc.exitSlow, MinStrength: tc.minStrength,
VWAPPeriod: 200, RegimeFilter: tc.regime,
}
cfg.TrailATRMult = tc.trailATR

res := backtest.Run(sym, cs, cfg)
m := res.Metrics

score := m.ProfitFactor
if m.GrossLoss == 0 && m.GrossProfit > 0 {
score = 999
}
if m.TotalTrades > 0 {
score = score * math.Sqrt(float64(m.TotalTrades)) * (1 - m.MaxDrawdownPct/200)
} else {
score = 0
}

pfDisplay := m.ProfitFactor
if m.GrossLoss == 0 && m.GrossProfit > 0 {
pfDisplay = 999
}

fmt.Printf("%-18s: trades=%3d WR=%5.1f%% PF=%5.2f PnL=%8.2f DD=%5.2f%% SQN=%.2f Score=%.2f\n",
tc.name, m.TotalTrades, m.WinRate, pfDisplay, m.NetPnL, m.MaxDrawdownPct, m.SQN, score)

if score > bestScore && m.TotalTrades >= 3 {
bestScore = score
bestName = tc.name
bestRes = res
}
}

if bestRes.Metrics.TotalTrades > 0 {
fmt.Printf("\n★ BEST: %s con Score=%.2f, PF=%.2f, PnL=%.2f, Trades=%d, DD=%.2f%%, WR=%.1f%%\n",
bestName, bestScore, bestRes.Metrics.ProfitFactor, bestRes.Metrics.NetPnL,
bestRes.Metrics.TotalTrades, bestRes.Metrics.MaxDrawdownPct, bestRes.Metrics.WinRate)
bestOverall[sym] = bestRes
bestOverallScore[sym] = bestScore
}
}

fmt.Println("\n============================================================")
fmt.Println("RIEPILOGO OTTIMIZZAZIONE PER SIMBOLO")
fmt.Println("============================================================")
totalPnL := 0.0
for _, sym := range symbols {
if res, ok := bestOverall[sym]; ok {
m := res.Metrics
totalPnL += m.NetPnL
fmt.Printf("%-10s: %-18s PnL=%8.2f  PF=%.2f  DD=%5.2f%%  Trades=%d\n",
sym, "BEST", m.NetPnL, m.ProfitFactor, m.MaxDrawdownPct, m.TotalTrades)
}
}
fmt.Printf("%-10s: %8.2f\n", "TOTALE", totalPnL)
}
