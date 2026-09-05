package main

import (
"encoding/json"
"fmt"
"io/ioutil"
"math"
"path/filepath"

"quadscalping/internal/backtest"
"quadscalping/internal/config"
"quadscalping/internal/market"
)

type OptimizeResult struct {
Symbol       string  `json:"symbol"`
StopATR      float64 `json:"stop_atr"`
TP1R         float64 `json:"tp1r"`
RegimeFilter string  `json:"regime_filter"`
UseAI        bool    `json:"use_ai"`
PnL          float64 `json:"pnl"`
WinRate      float64 `json:"win_rate"`
ProfitFactor float64 `json:"profit_factor"`
MaxDD        float64 `json:"max_dd"`
Sharpe       float64 `json:"sharpe"`
Score        float64 `json:"score"`
Trades       int     `json:"trades"`
}

func calculateScore(pnl, wr, pf, dd, sharpe float64, trades int) float64 {
if trades < 20 || dd > 25.0 {
return -999999
}
tradeBonus := math.Min(float64(trades)/100.0, 2.0)
ddPenalty := math.Max(dd-5.0, 0) * 2.0
return pnl*0.5 + pf*50.0 + sharpe*30.0 - ddPenalty + tradeBonus*10.0
}

func optimizeSymbol(symbol string, candles []market.Candle) OptimizeResult {
best := OptimizeResult{Symbol: symbol, Score: -999999}

stopValues := []float64{1.5, 2.0, 2.5, 3.0, 3.5, 4.0}
tpValues := []float64{1.5, 2.0, 2.5, 3.0, 4.0, 5.0, 6.0}
regimes := []string{"any", "down", "up"}
useAI := []bool{false, true}

for _, stop := range stopValues {
for _, tp := range tpValues {
for _, regime := range regimes {
for _, ai := range useAI {
cfg := config.LoadConfig("config.json")
cfg.HPS.StopATR = stop
cfg.HPS.TP1R = tp
cfg.HPS.RegimeFilter = regime
cfg.HPS.UseAIRegime = ai
cfg.HPS.AIRegimeFilter = "normal"

strat := backtest.NewStrategyFromConfig(cfg.HPS)
bt := backtest.NewBacktest(strat, 10000, 0.001, 0.001)

results, _ := bt.Run(candles)
if len(results.Trades) == 0 {
continue
}

pf := results.ProfitFactor
if math.IsInf(pf, 0) || math.IsNaN(pf) {
pf = 0
}

score := calculateScore(results.NetPnL, results.WinRate, pf, results.MaxDrawdown, results.SharpeRatio, len(results.Trades))

if score > best.Score {
best = OptimizeResult{
Symbol:       symbol,
StopATR:      stop,
TP1R:         tp,
RegimeFilter: regime,
UseAI:        ai,
PnL:          results.NetPnL,
WinRate:      results.WinRate * 100,
ProfitFactor: pf,
MaxDD:        results.MaxDrawdown,
Sharpe:       results.SharpeRatio,
Score:        score,
Trades:       len(results.Trades),
}
}
}
}
}
}

return best
}

func loadCandlesFromCSV(filename string) ([]market.Candle, error) {
file, err := ioutil.ReadFile(filename)
if err != nil {
return nil, err
}

lines := strings.Split(string(file), "\n")
var candles []market.Candle

for i, line := range lines {
if i == 0 || line == "" {
continue
}
parts := strings.Split(line, ",")
if len(parts) < 6 {
continue
}

timeMs, _ := strconv.ParseInt(parts[0], 10, 64)
open, _ := strconv.ParseFloat(parts[1], 64)
high, _ := strconv.ParseFloat(parts[2], 64)
low, _ := strconv.ParseFloat(parts[3], 64)
close, _ := strconv.ParseFloat(parts[4], 64)
volume, _ := strconv.ParseFloat(parts[5], 64)

candles = append(candles, market.Candle{
Time:   timeMs,
Open:   open,
High:   high,
Low:    low,
Close:  close,
Volume: volume,
})
}

return candles, nil
}

func main() {
symbols := []string{"BTCUSDT", "ETHUSDT", "DOGEUSDT"}
var allResults []OptimizeResult

fmt.Println("=== Ottimizzazione H1 con Dati Reali (1 Anno) ===\n")

for _, symbol := range symbols {
filename := filepath.Join("data", fmt.Sprintf("%s_1h.csv", symbol))
candles, err := loadCandlesFromCSV(filename)
if err != nil {
fmt.Printf("Errore caricamento %s: %v\n", symbol, err)
continue
}

fmt.Printf("Ottimizzazione %s (%d candele)...\n", symbol, len(candles))
result := optimizeSymbol(symbol, candles)
allResults = append(allResults, result)

fmt.Printf("  StopATR: %.1f, TP1R: %.1f, Regime: %s, AI: %v\n",
result.StopATR, result.TP1R, result.RegimeFilter, result.UseAI)
fmt.Printf("  Trades: %d, WinRate: %.1f%%, PF: %.2f, PnL: %.2f, MaxDD: %.2f%%, Sharpe: %.2f\n",
result.Trades, result.WinRate, result.ProfitFactor, result.PnL, result.MaxDD, result.Sharpe)
fmt.Printf("  Score: %.2f\n\n", result.Score)
}

jsonData, _ := json.MarshalIndent(allResults, "", "  ")
ioutil.WriteFile("reports/h1_optimization_results.json", jsonData, 0644)

totalPnL := 0.0
for _, r := range allResults {
totalPnL += r.PnL
}
fmt.Printf("\n=== PnL Totale Ottimizzato: %.2f USDT ===\n", totalPnL)
fmt.Println("Risultati salvati in reports/h1_optimization_results.json")
}
