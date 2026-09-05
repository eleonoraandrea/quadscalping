// Command backtest esegue il backtest HPS sui dati in cache (scaricandoli
// se mancanti) e produce un report HTML.
//
// Usage:
//
//	go run ./cmd/backtest -config config.json -out reports/report.html
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"quadscalping/internal/backtest"
	"quadscalping/internal/config"
	"quadscalping/internal/data"
	"quadscalping/internal/market"
	"quadscalping/internal/report"
)

func main() {
	var (
		cfgPath  = flag.String("config", "config.json", "file di configurazione")
		symbols  = flag.String("symbols", "", "simboli override (CSV, es. BTCUSDT,ETHUSDT)")
		interval = flag.String("interval", "", "timeframe override (es. 5m)")
		out      = flag.String("out", "reports/report.html", "report HTML di output")
	)
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	syms := cfg.Trading.Symbols
	if *symbols != "" {
		syms = splitCSV(*symbols)
	}
	tf := cfg.Trading.Timeframe
	if *interval != "" {
		tf = *interval
	}
	if _, ok := market.Minutes(tf); !ok {
		log.Fatalf("timeframe non valido: %s", tf)
	}

	ctx := context.Background()
	client := data.NewClient()

	var reps []report.SymbolReport
	for _, sym := range syms {
		path := fmt.Sprintf("%s/%s_%s.csv", cfg.Bot.DataDir, sym, tf)
		cs, err := data.UpdateCSV(ctx, path, client, sym, tf)
		if err != nil {
			log.Fatalf("dati %s: %v", sym, err)
		}
		if len(cs) < 300 {
			log.Fatalf("%s: solo %d candele, servono almeno 300", sym, len(cs))
		}
		bcfg := cfg.BacktestConfigFor(sym)
		if m, ok := market.Minutes(tf); ok {
			bcfg.TimeframeMinutes = m
		}
		res := backtest.Run(sym, cs, bcfg)
		m := res.Metrics
		pf := fmt.Sprintf("%.2f", m.ProfitFactor)
		if m.GrossLoss == 0 && m.GrossProfit > 0 {
			pf = "∞"
		}
		fmt.Printf("%-12s candele=%-7d trade=%-5d winrate=%5.1f%% PF=%-5s PnL=%10.2f maxDD=%6.2f%% SQN=%.2f sharpe=%.2f\n",
			sym, len(cs), m.TotalTrades, m.WinRate, pf, m.NetPnL, m.MaxDrawdownPct, m.SQN, m.Sharpe)
		reps = append(reps, report.SymbolReport{
			Symbol: sym, Timeframe: tf,
			From: time.UnixMilli(cs[0].Time), To: time.UnixMilli(cs[len(cs)-1].Time),
			Result: res,
		})
	}

	if err := report.WriteFile(*out, report.Data{
		Title:       "HPS Quad Super Signal — Backtest",
		GeneratedAt: time.Now(),
		Symbols:     reps,
	}); err != nil {
		log.Fatalf("report: %v", err)
	}
	fmt.Printf("✓ report: %s\n", *out)
}

func splitCSV(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' || r == ' ' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
