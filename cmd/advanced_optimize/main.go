// Package main esegue una grid search avanzata con i nuovi parametri dinamici.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"quadscalping/internal/backtest"
	"quadscalping/internal/data"
	"quadscalping/internal/market"
	"quadscalping/internal/strategy"
)

type optResult struct {
	Symbol       string
	Params       strategy.Params
	NetPnL       float64
	WinRate      float64
	MaxDD        float64
	Sharpe       float64
	TotalTrades  int
	ProfitFactor float64
}

func runOptimization(symbol string, candles []market.Candle) optResult {
	baseParams := strategy.DefaultParams()
	
	// Griglia di parametri avanzati da testare
	trailingStopValues := []float64{0, 1.5, 2.0, 2.5}
	dynamicTPValues := []bool{false, true}
	tp1RValues := []float64{1.5, 2.0, 3.0, 5.0}
	stopATRValues := []float64{1.5, 2.0, 2.5, 3.0}
	regimeFilters := []string{"down", "any"}
	
	var bestResult optResult
	bestScore := -1e9
	
	cfg := backtest.Config{
		InitialCapital:   10000,
		RiskPct:          0.01,
		Commission:       0.0004,
		Slippage:         0.0001,
		MaxLeverage:      1.0,
		CooldownBars:     10,
		TimeframeMinutes: 240, // 4h
		VolAdjust:        true,
		StrengthSizing:   true,
		DDThrottlePct:    0.15,
		TrailATRMult:     0, // gestito dai parametri della strategia
	}
	
	totalCombos := len(trailingStopValues) * len(dynamicTPValues) * len(tp1RValues) * 
		len(stopATRValues) * len(regimeFilters)
	
	combo := 0
	for _, trail := range trailingStopValues {
		for _, dynTP := range dynamicTPValues {
			for _, tp1r := range tp1RValues {
				for _, stop := range stopATRValues {
					for _, regime := range regimeFilters {
						combo++
						if combo%50 == 0 {
							fmt.Printf("\r%s: %d/%d combinazioni...", symbol, combo, totalCombos)
						}
						
						p := baseParams
						p.TrailingStopATR = trail
						p.DynamicTP = dynTP
						p.TP1R = tp1r
						p.StopATR = stop
						p.RegimeFilter = regime
						cfg.Params = p
						
						res := backtest.Run(symbol, candles, cfg)
						
						// Score function: PnL penalizzato per drawdown alto
						score := res.Metrics.NetPnL
						if res.Metrics.MaxDrawdownPct > 10 {
							score *= 0.5
						}
						if res.Metrics.ProfitFactor > 0 && res.Metrics.TotalTrades >= 3 {
							score += res.Metrics.ProfitFactor * 100
						}
						score += res.Metrics.Sharpe * 50
						
						if score > bestScore && res.Metrics.TotalTrades >= 3 {
							bestScore = score
							bestResult = optResult{
								Symbol:       symbol,
								Params:       p,
								NetPnL:       res.Metrics.NetPnL,
								WinRate:      res.Metrics.WinRate,
								MaxDD:        res.Metrics.MaxDrawdownPct,
								Sharpe:       res.Metrics.Sharpe,
								TotalTrades:  res.Metrics.TotalTrades,
								ProfitFactor: res.Metrics.ProfitFactor,
							}
						}
					}
				}
			}
		}
	}
	
	fmt.Printf("\r%s: completato %d combinazioni. Best PnL: %.2f\n", symbol, totalCombos, bestResult.NetPnL)
	return bestResult
}

func main() {
	symbols := []string{"BTCUSDT", "ETHUSDT", "DOGEUSDT"}
	timeframe := "4h"
	end := time.Now()
	start := end.AddDate(0, -18, 0) // 18 mesi di dati
	
	fmt.Println("=== Ottimizzazione Avanzata Parametri ===")
	fmt.Printf("Periodo: %s - %s\n", start.Format("2006-01-02"), end.Format("2006-01-02"))
	fmt.Printf("Timeframe: %s\n\n", timeframe)
	
	var results []optResult
	
	for _, sym := range symbols {
		fmt.Printf("Caricamento dati per %s...\n", sym)
		
		// Costruisci percorso file CSV
		dataDir := "data"
		filename := fmt.Sprintf("%s_%s.csv", sym, timeframe)
		path := filepath.Join(dataDir, filename)
		
		candles, err := data.LoadCSV(path)
		if err != nil {
			fmt.Printf("Errore caricamento %s: %v\n", sym, err)
			continue
		}
		fmt.Printf("  Trovate %d candele\n", len(candles))
		
		result := runOptimization(sym, candles)
		results = append(results, result)
		fmt.Printf("\n>>> Migliori parametri per %s:\n", sym)
		fmt.Printf("    TrailingStopATR: %.1f\n", result.Params.TrailingStopATR)
		fmt.Printf("    DynamicTP: %v\n", result.Params.DynamicTP)
		fmt.Printf("    TP1R: %.1f\n", result.Params.TP1R)
		fmt.Printf("    StopATR: %.1f\n", result.Params.StopATR)
		fmt.Printf("    RegimeFilter: %s\n", result.Params.RegimeFilter)
		fmt.Printf("    PnL: %.2f USDT\n", result.NetPnL)
		fmt.Printf("    WinRate: %.1f%%\n", result.WinRate)
		fmt.Printf("    MaxDD: %.2f%%\n", result.MaxDD)
		fmt.Printf("    Sharpe: %.2f\n", result.Sharpe)
		fmt.Printf("    Trades: %d\n", result.TotalTrades)
		fmt.Printf("    ProfitFactor: %.2f\n\n", result.ProfitFactor)
	}
	
	// Summary finale
	fmt.Println("\n=== RIEPILOGO OTTIMIZZAZIONE ===")
	totalPnL := 0.0
	for _, r := range results {
		fmt.Printf("%-10s PnL: %+8.2f  WR: %5.1f%%  DD: %5.2f%%  Sharpe: %6.2f  Trades: %3d\n",
			r.Symbol, r.NetPnL, r.WinRate, r.MaxDD, r.Sharpe, r.TotalTrades)
		totalPnL += r.NetPnL
	}
	fmt.Printf("%-10s PnL: %+8.2f (TOTALE)\n", "TOTALE", totalPnL)
	
	// Salva risultati in file
	f, err := os.Create("reports/advanced_optimization.txt")
	if err != nil {
		fmt.Printf("Errore creazione file report: %v\n", err)
		return
	}
	defer f.Close()
	
	fmt.Fprintf(f, "Ottimizzazione Avanzata - %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(f, "Periodo: %s - %s\n\n", start.Format("2006-01-02"), end.Format("2006-01-02"))
	
	for _, r := range results {
		fmt.Fprintf(f, "\n=== %s ===\n", r.Symbol)
		fmt.Fprintf(f, "TrailingStopATR: %.1f\n", r.Params.TrailingStopATR)
		fmt.Fprintf(f, "DynamicTP: %v\n", r.Params.DynamicTP)
		fmt.Fprintf(f, "TP1R: %.1f\n", r.Params.TP1R)
		fmt.Fprintf(f, "StopATR: %.1f\n", r.Params.StopATR)
		fmt.Fprintf(f, "RegimeFilter: %s\n", r.Params.RegimeFilter)
		fmt.Fprintf(f, "PnL: %.2f\n", r.NetPnL)
		fmt.Fprintf(f, "WinRate: %.1f\n", r.WinRate)
		fmt.Fprintf(f, "MaxDD: %.2f\n", r.MaxDD)
		fmt.Fprintf(f, "Sharpe: %.2f\n", r.Sharpe)
		fmt.Fprintf(f, "Trades: %d\n", r.TotalTrades)
		fmt.Fprintf(f, "ProfitFactor: %.2f\n", r.ProfitFactor)
	}
	
	fmt.Printf("\nReport salvato in reports/advanced_optimization.txt\n")
}
