// Command sweep esegue una griglia di backtest HPS con split
// in-sample / out-of-sample e stampa i risultati ordinati per OOS.
//
// Usage:
//
//	go run ./cmd/sweep -symbol BTCUSDT -intervals 15m,1h,4h
package main

import (
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"

	"quadscalping/internal/backtest"
	"quadscalping/internal/config"
	"quadscalping/internal/data"
	"quadscalping/internal/market"
	"quadscalping/internal/strategy"
)

// Combo è una configurazione della griglia.
type Combo struct {
	Interval     string
	StopATR      float64
	TP1R         float64
	TP2R         float64
	ExitSlow     float64
	RegimeFilter string
	MinStrength  float64
}

// Grid genera il prodotto cartesiano dei parametri.
func Grid(intervals []string, stopATRs, tp1Rs, tp2Rs, exitSlows []float64, regimes []string, strengths []float64) []Combo {
	var out []Combo
	for _, iv := range intervals {
		for _, sa := range stopATRs {
			for _, tp := range tp1Rs {
				for _, tp2 := range tp2Rs {
					for _, es := range exitSlows {
						for _, rf := range regimes {
							for _, ms := range strengths {
								out = append(out, Combo{iv, sa, tp, tp2, es, rf, ms})
							}
						}
					}
				}
			}
		}
	}
	return out
}

// SplitIS divide le candele in-sample (prime frac) e out-of-sample.
func SplitIS(cs []market.Candle, frac float64) (is, oos []market.Candle) {
	n := int(float64(len(cs)) * frac)
	return cs[:n], cs[n:]
}

func runBT(symbol string, cs []market.Candle, c Combo, zero bool) backtest.Metrics {
	cfg := backtest.DefaultConfig()
	p := strategy.DefaultParams()
	p.StopATR = c.StopATR
	p.TP1R = c.TP1R
	p.RegimeFilter = c.RegimeFilter
	p.MinStrength = c.MinStrength
	p.TP2R = c.TP2R
	p.ExitSlow = c.ExitSlow
	cfg.Params = p
	if m, ok := market.Minutes(c.Interval); ok {
		cfg.TimeframeMinutes = m
	}
	if zero {
		cfg.Commission = 0
		cfg.Slippage = 0
	}
	res := backtest.Run(symbol, cs, cfg)
	return res.Metrics
}

// riskVariant è una configurazione di money management da confrontare.
type riskVariant struct {
	name  string
	apply func(c *backtest.Config)
}

func riskVariants() []riskVariant {
	set := func(f func(c *backtest.Config)) func(c *backtest.Config) { return f }
	return []riskVariant{
		{"base", set(func(c *backtest.Config) {})},
		{"vol", set(func(c *backtest.Config) { c.VolAdjust = true })},
		{"str", set(func(c *backtest.Config) { c.StrengthSizing = true })},
		{"volstr", set(func(c *backtest.Config) { c.VolAdjust = true; c.StrengthSizing = true })},
		{"volstr_dd10", set(func(c *backtest.Config) {
			c.VolAdjust, c.StrengthSizing, c.DDThrottlePct = true, true, 0.10
		})},
		{"volstr_trail2", set(func(c *backtest.Config) {
			c.VolAdjust, c.StrengthSizing, c.TrailATRMult = true, true, 2.0
		})},
		{"volstr_dd10_trail2", set(func(c *backtest.Config) {
			c.VolAdjust, c.StrengthSizing, c.DDThrottlePct, c.TrailATRMult = true, true, 0.10, 2.0
		})},
		{"volstr_dd10_trail2_streak3", set(func(c *backtest.Config) {
			c.VolAdjust, c.StrengthSizing, c.DDThrottlePct = true, true, 0.10
			c.TrailATRMult, c.LossStreakN, c.LossStreakPauseBars = 2.0, 3, 24
		})},
		{"volstr_dd10_trail3", set(func(c *backtest.Config) {
			c.VolAdjust, c.StrengthSizing, c.DDThrottlePct, c.TrailATRMult = true, true, 0.10, 3.0
		})},
		{"dd10_solo", set(func(c *backtest.Config) { c.DDThrottlePct = 0.10 })},
	}
}

// riskMain confronta le varianti di money management sui parametri base
// del config per un simbolo, con validazione a terzi.
func riskMain(cfgPath, symbol, intervals string) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	for _, iv := range strings.Split(intervals, ",") {
		cs, err := data.LoadCSV(fmt.Sprintf("data/%s_%s.csv", symbol, iv))
		if err != nil {
			log.Fatalf("%s %s: %v", symbol, iv, err)
		}
		base := cfg.BacktestConfigFor(symbol)
		if m, ok := market.Minutes(iv); ok {
			base.TimeframeMinutes = m
		}
		n3 := len(cs) / 3

		fmt.Printf("\n=== %s %s: varianti money management (terzi indipendenti) ===\n", symbol, iv)
		fmt.Println("variante                 | pf1(tr)  pf2(tr)  pf3(tr)  | cumPnL   maxDD   Calmar  | robusta")
		for _, v := range riskVariants() {
			bc := base
			v.apply(&bc)
			var pf [3]float64
			var tr [3]int
			ok3 := true
			for k := 0; k < 3; k++ {
				part := cs[k*n3 : (k+1)*n3]
				if k == 2 {
					part = cs[2*n3:]
				}
				m := backtest.Run(symbol, part, bc).Metrics
				pf[k], tr[k] = m.ProfitFactor, m.TotalTrades
				if !(m.NetPnL > 0 && m.TotalTrades >= 8) {
					ok3 = false
				}
			}
			full := backtest.Run(symbol, cs, bc).Metrics
			calmar := 0.0
			if full.MaxDrawdownPct > 0 {
				calmar = (full.NetPnL / cfg.Bot.InitialCapital) / (full.MaxDrawdownPct / 100)
			}
			rob := "NO"
			if ok3 {
				rob = "SI"
			}
			fmt.Printf("%-24s | %.2f(%d)  %.2f(%d)  %.2f(%d)  | %+8.0f  %5.1f%%  %5.2f   | %s\n",
				v.name, pf[0], tr[0], pf[1], tr[1], pf[2], tr[2],
				full.NetPnL, full.MaxDrawdownPct, calmar, rob)
		}
	}
}

func main() {
	var (
		symbol    = flag.String("symbol", "BTCUSDT", "simbolo")
		intervals = flag.String("intervals", "15m,1h,4h", "timeframe CSV")
		minOOS    = flag.Int("min-oos-trades", 25, "trade minimi OOS per candidati validi")
		zeroDiag  = flag.Bool("zero-cost-diag", false, "stampa diagnostica senza commissioni/slippage sul 5m")
		thirds    = flag.Bool("thirds", false, "validazione robustezza: 3 terzi del periodo, PF per terzo")
		risk      = flag.Bool("risk", false, "confronta varianti di money management sui parametri del config")
		cfgPath   = flag.String("config", "config.json", "config con parametri base e symbol_overrides")
	)
	flag.Parse()

	if *risk {
		riskMain(*cfgPath, *symbol, *intervals)
		return
	}

	ivs := strings.Split(*intervals, ",")
	for _, iv := range ivs {
		if _, ok := market.Minutes(iv); !ok {
			log.Fatalf("timeframe non valido: %s", iv)
		}
	}

	if *zeroDiag {
		diagTF := ivs[0]
		cs, err := data.LoadCSV(fmt.Sprintf("data/%s_%s.csv", *symbol, diagTF))
		if err != nil {
			log.Fatal(err)
		}
		base := Combo{diagTF, 1.5, 1.5, 0, 70, "down", 50}
		withCost := runBT(*symbol, cs, base, false)
		noCost := runBT(*symbol, cs, base, true)
		fmt.Printf("[diag %s default] con costi: trades=%d PF=%.2f PnL=%.0f | senza costi: trades=%d PF=%.2f PnL=%.0f\n",
			diagTF,
			withCost.TotalTrades, withCost.ProfitFactor, withCost.NetPnL,
			noCost.TotalTrades, noCost.ProfitFactor, noCost.NetPnL)
	}

	type row struct {
		Combo
		ISPF, OOSPF   float64
		ISTr, OOSTr   int
		OOSPnL        float64
		OOSWin, OOSDD float64
	}
	var rows []row

	type thRow struct {
		Combo
		PF  [3]float64
		Tr  [3]int
		PnL [3]float64
	}
	var thRows []thRow

	for _, iv := range ivs {
		cs, err := data.LoadCSV(fmt.Sprintf("data/%s_%s.csv", *symbol, iv))
		if err != nil {
			log.Fatalf("%s: %v (scaricalo con cmd/download)", iv, err)
		}
		is, oos := SplitIS(cs, 0.7)

		for _, c := range Grid([]string{iv},
			[]float64{1.5, 2.0, 3.0},
			[]float64{1.0, 1.5, 2.0},
			[]float64{0, 3.0},
			[]float64{60, 80},
			[]string{"down", "up", "any"},
			[]float64{50, 70}) {

			if *thirds {
				// robustezza: tre terzi indipendenti (ognuno con warmup)
				n3 := len(cs) / 3
				var r3 thRow
				r3.Combo = c
				for k := 0; k < 3; k++ {
					part := cs[k*n3 : (k+1)*n3]
					if k == 2 {
						part = cs[2*n3:]
					}
					m := runBT(*symbol, part, c, false)
					r3.PF[k], r3.Tr[k], r3.PnL[k] = m.ProfitFactor, m.TotalTrades, m.NetPnL
				}
				thRows = append(thRows, r3)
				continue
			}
			ism := runBT(*symbol, is, c, false)
			oosm := runBT(*symbol, oos, c, false)
			rows = append(rows, row{c, ism.ProfitFactor, oosm.ProfitFactor,
				ism.TotalTrades, oosm.TotalTrades, oosm.NetPnL, oosm.WinRate, oosm.MaxDrawdownPct})
		}
	}

	if *thirds {
		type cand struct {
			thRow
			okThirds int
			cum      float64
		}
		var cands2 []cand
		for _, r := range thRows {
			ok, cum := 0, 0.0
			for k := 0; k < 3; k++ {
				cum += r.PnL[k]
				if r.PnL[k] > 0 && r.Tr[k] >= 8 {
					ok++
				}
			}
			if ok == 3 && cum > 0 {
				cands2 = append(cands2, cand{r, ok, cum})
			}
		}
		sort.Slice(cands2, func(i, j int) bool { return cands2[i].cum > cands2[j].cum })
		fmt.Printf("\n=== TERZI: profittevoli in TUTTI e 3 i terzi (>=8 trade/terzo) ===\n")
		fmt.Println("tf    stop tp1 tp2 es  regime str  | pf1(tr)   pf2(tr)   pf3(tr)   | cum PnL")
		for i, r := range cands2 {
			if i >= 20 {
				break
			}
			fmt.Printf("%-5s %.1f %.1f %.0f  %.0f  %-5s %.0f   | %.2f(%d)  %.2f(%d)  %.2f(%d)  | %+.0f\n",
				r.Interval, r.StopATR, r.TP1R, r.TP2R, r.ExitSlow, r.RegimeFilter, r.MinStrength,
				r.PF[0], r.Tr[0], r.PF[1], r.Tr[1], r.PF[2], r.Tr[2], r.cum)
		}
		if len(cands2) == 0 {
			fmt.Println("nessuna combinazione profittevole in tutti i terzi")
		}
		fmt.Printf("\ntotale combinazioni: %d, robuste 3/3: %d\n", len(thRows), len(cands2))
		return
	}

	// candidati: abbastanza trade OOS e OOS in profitto
	var cands []row
	for _, r := range rows {
		if r.OOSTr >= *minOOS && r.OOSPnL > 0 && r.OOSPF > 1 {
			cands = append(cands, r)
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].OOSPF != cands[j].OOSPF {
			return cands[i].OOSPF > cands[j].OOSPF
		}
		return cands[i].OOSPnL > cands[j].OOSPnL
	})

	// diagnostica: migliori per PnL OOS assoluto (anche in perdita)
	byPnL := append([]row{}, rows...)
	sort.Slice(byPnL, func(i, j int) bool { return byPnL[i].OOSPnL > byPnL[j].OOSPnL })
	fmt.Println("\n=== TOP 10 per PnL OOS assoluto (diagnostica, anche negativi) ===")
	for i, r := range byPnL {
		if i >= 10 {
			break
		}
		fmt.Printf("%-5s stop=%.1f tp1=%.1f tp2=%.0f es=%.0f %-4s str=%.0f | OOSpf=%.2f tr=%d pnl=%+.0f ISpf=%.2f\n",
			r.Interval, r.StopATR, r.TP1R, r.TP2R, r.ExitSlow, r.RegimeFilter, r.MinStrength,
			r.OOSPF, r.OOSTr, r.OOSPnL, r.ISPF)
	}

	fmt.Printf("\n=== TOP 15 per OOS profit factor (min %d trade OOS, OOS in profitto) ===\n", *minOOS)
	fmt.Println("tf    stop tp1 tp2 es  regime str  ISpf  IStr  OOSpf Oostr OOSwin OOSdd    OOSPnL")
	for i, r := range cands {
		if i >= 15 {
			break
		}
		fmt.Printf("%-5s %.1f %.1f %.0f  %.0f  %-5s %.0f   %.2f  %-5d %.2f  %-5d %.1f%%  %.1f%%  %+.0f\n",
			r.Interval, r.StopATR, r.TP1R, r.TP2R, r.ExitSlow, r.RegimeFilter, r.MinStrength,
			r.ISPF, r.ISTr, r.OOSPF, r.OOSTr, r.OOSWin, r.OOSDD, r.OOSPnL)
	}
	if len(cands) == 0 {
		fmt.Println("nessun candidato OOS profittevole")
	}
	fmt.Printf("\ntotale combinazioni: %d, candidate valide: %d\n", len(rows), len(cands))
}
