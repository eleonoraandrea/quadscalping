// Command final_backtest esegue il backtest con parametri ottimizzati
package main

import (
"fmt"
"log"
"time"
"quadscalping/internal/backtest"
"quadscalping/internal/data"
"quadscalping/internal/report"
"quadscalping/internal/strategy"
)

func main() {
symbols := []string{"BTCUSDT", "ETHUSDT", "DOGEUSDT"}
tf := "4h"

var reps []report.SymbolReport

optParams := map[string]strategy.Params{
"BTCUSDT": {
QuadFast: 30, QuadSmall: 30, QuadMid: 30, QuadLong: 40,
StopMode: "atr", StopATR: 1.5, StopLookback: 20, StopBufferATR: 0.1,
TP1R: 5.0, TP2R: 0, PartialPct: 0.5, BreakevenOnTP1: true,
ExitSlow: 70, MinStrength: 50,
VWAPPeriod: 200, RegimeFilter: "down",
},
"ETHUSDT": {
QuadFast: 30, QuadSmall: 30, QuadMid: 30, QuadLong: 40,
StopMode: "atr", StopATR: 1.5, StopLookback: 20, StopBufferATR: 0.1,
TP1R: 1.5, TP2R: 0, PartialPct: 0.5, BreakevenOnTP1: true,
ExitSlow: 80, MinStrength: 50,
VWAPPeriod: 200, RegimeFilter: "any",
},
"DOGEUSDT": {
QuadFast: 30, QuadSmall: 30, QuadMid: 30, QuadLong: 40,
StopMode: "atr", StopATR: 3.0, StopLookback: 20, StopBufferATR: 0.1,
TP1R: 1.5, TP2R: 0, PartialPct: 0.5, BreakevenOnTP1: true,
ExitSlow: 70, MinStrength: 50,
VWAPPeriod: 200, RegimeFilter: "down",
},
}

for _, sym := range symbols {
path := fmt.Sprintf("data/%s_%s.csv", sym, tf)
cs, err := data.LoadCSV(path)
if err != nil {
log.Fatalf("Errore load %s: %v", sym, err)
}

cfg := backtest.DefaultConfig()
cfg.InitialCapital = 10000
cfg.RiskPct = 0.01
cfg.Params = optParams[sym]
cfg.TimeframeMinutes = 240

res := backtest.Run(sym, cs, cfg)
m := res.Metrics

pf := fmt.Sprintf("%.2f", m.ProfitFactor)
if m.GrossLoss == 0 && m.GrossProfit > 0 {
pf = "inf"
}

fmt.Printf("%-12s candele=%-7d trade=%-5d winrate=%5.1f%% PF=%-5s PnL=%10.2f maxDD=%6.2f%% SQN=%.2f sharpe=%.2f\n",
sym, len(cs), m.TotalTrades, m.WinRate, pf, m.NetPnL, m.MaxDrawdownPct, m.SQN, m.Sharpe)

reps = append(reps, report.SymbolReport{
Symbol: sym, Timeframe: tf,
From: time.UnixMilli(cs[0].Time), To: time.UnixMilli(cs[len(cs)-1].Time),
Result: res,
})
}

out := "reports/optimized_report.html"
if err := report.WriteFile(out, report.Data{
Title:       "HPS Quad Super Signal — Backtest Ottimizzato",
GeneratedAt: time.Now(),
Symbols:     reps,
}); err != nil {
log.Fatalf("report: %v", err)
}
fmt.Printf("✓ report: %s\n", out)

totalPnL := 0.0
for _, r := range reps {
totalPnL += r.Result.Metrics.NetPnL
}
fmt.Printf("\n=== TOTALE COMPLESSIVO: %.2f USDT ===\n", totalPnL)
}
