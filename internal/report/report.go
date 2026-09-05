// Package report genera il report HTML self-contained del backtest.
package report

import (
	"fmt"
	"html/template"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"quadscalping/internal/backtest"
)

// SymbolReport è il risultato di un simbolo dentro il report.
type SymbolReport struct {
	Symbol    string
	Timeframe string
	From, To  time.Time
	Result    backtest.Result
}

// Data è l'input completo del report.
type Data struct {
	Title       string
	GeneratedAt time.Time
	Symbols     []SymbolReport
}

// ---------- serie e scale ----------

// NiceTicks ritorna tick "puliti" per un range (per gli assi y).
func NiceTicks(min, max float64, target int) []float64 {
	if target < 2 {
		target = 2
	}
	if math.IsNaN(min) || math.IsNaN(max) || min > max {
		return []float64{0, 1}
	}
	if max-min < 1e-12 {
		// range degenere: espandi simmetricamente
		pad := math.Abs(min)*0.1 + 1
		min, max = min-pad, max+pad
	}
	rawStep := (max - min) / float64(target)
	mag := math.Pow(10, math.Floor(math.Log10(rawStep)))
	norm := rawStep / mag
	var step float64
	switch {
	case norm <= 1:
		step = 1
	case norm <= 2:
		step = 2
	case norm <= 2.5:
		step = 2.5
	case norm <= 5:
		step = 5
	default:
		step = 10
	}
	step *= mag
	start := math.Floor(min/step) * step
	end := math.Ceil(max/step) * step
	var out []float64
	for v := start; v <= end+step/2; v += step {
		out = append(out, math.Round(v/step)*step)
		if len(out) > 100 {
			break
		}
	}
	return out
}

// Downsample riduce una serie a massimo maxPoints punti
// (primo e ultimo sempre preservati).
func Downsample(xs []float64, maxPoints int) []float64 {
	if maxPoints <= 0 || len(xs) <= maxPoints {
		return append([]float64{}, xs...)
	}
	out := make([]float64, 0, maxPoints)
	step := float64(len(xs)-1) / float64(maxPoints-1)
	for i := 0; i < maxPoints; i++ {
		out = append(out, xs[int(math.Round(float64(i)*step))])
	}
	// forza gli estremi esatti
	out[0] = xs[0]
	out[len(out)-1] = xs[len(xs)-1]
	return out
}

// ---------- formatting helpers ----------

func fmtMoney(v float64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	// formato italiano: migliaia con punto, decimali con virgola
	return sign + "$" + groupInt(int64(math.Round(v))) + "," + fmt.Sprintf("%02d", int(math.Mod(math.Round(v*100), 100)))
}

func groupInt(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, ".")
}

func fmtNum(v float64, dec int) string {
	return strconv.FormatFloat(v, 'f', dec, 64)
}

func fmtTime(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("02-01-2006 15:04")
}

// ---------- view model ----------

type point struct {
	T float64 `json:"t"`
	V float64 `json:"v"`
}

type monthlyPnL struct {
	Month string
	PnL   float64
}

type symbolVM struct {
	Symbol    string
	Timeframe string
	Period    string
	M         backtest.Metrics
	NetPnL    string
	EquityPts []point
	DDPts     []point
	EquitySVG template.HTML
	DDSVG     template.HTML
	Monthly   []monthlyPnL
	MonthSVG  template.HTML
	TradeRows []tradeRow
	Cfg       backtest.Config
	FinalEq   string
}

type tradeRow struct {
	Idx                   int
	Entry, Exit, Motive   string
	EntryPx, ExitPx, Size string
	PnL, R                string
	PnlClass              string
	Partial               string
}

// ---------- generazione ----------

// GenerateHTML produce l'HTML completo (self-contained, niente dipendenze esterne).
func GenerateHTML(d Data) (string, error) {
	var vms []symbolVM
	for _, s := range d.Symbols {
		vms = append(vms, buildSymbolVM(s))
	}
	tpl, err := template.New("report").Funcs(template.FuncMap{
		"safe":  func(s string) template.HTML { return template.HTML(s) },
		"money": fmtMoney,
		"pnlcls": func(v float64) string {
			if v > 0 {
				return "pos"
			}
			if v < 0 {
				return "neg"
			}
			return ""
		},
		"mul": func(a, b float64) float64 { return a * b },
	}).Parse(reportTpl)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if err := tpl.Execute(&sb, map[string]interface{}{
		"Title":       d.Title,
		"GeneratedAt": d.GeneratedAt.Format("02-01-2006 15:04:05 MST"),
		"Symbols":     vms,
	}); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func buildSymbolVM(s SymbolReport) symbolVM {
	r := s.Result
	vm := symbolVM{
		Symbol:    s.Symbol,
		Timeframe: s.Timeframe,
		M:         r.Metrics,
		NetPnL:    fmtMoney(r.Metrics.NetPnL),
		FinalEq:   fmtMoney(last(r.Equity)),
		Cfg:       r.Config,
		Period: fmt.Sprintf("%s → %s",
			time.UnixMilli(firstT(r)).Format("02-01-2006"),
			time.UnixMilli(lastT(r)).Format("02-01-2006")),
	}

	// equity + drawdown (downsample per l'SVG, dati hover allineati)
	const maxPts = 900
	eq := r.Equity
	times := make([]float64, len(r.EquityTimes))
	for i, t := range r.EquityTimes {
		times[i] = float64(t)
	}
	if len(eq) > maxPts {
		idx := downsampleIndices(len(eq), maxPts)
		var e2, t2 []float64
		for _, i := range idx {
			e2 = append(e2, eq[i])
			t2 = append(t2, times[i])
		}
		eq, times = e2, t2
	}
	// drawdown % dal picco corrente
	peak := math.Inf(-1)
	dd := make([]float64, len(eq))
	for i, e := range eq {
		if e > peak {
			peak = e
		}
		if peak > 0 {
			dd[i] = (e/peak - 1) * 100
		}
	}
	vm.DDPts = zip(times, dd)
	vm.EquityPts = zip(times, eq)
	vm.EquitySVG = template.HTML(lineChart(vm.EquityPts, chartOpts{
		W: 860, H: 260, YFmt: func(v float64) string { return groupInt(int64(v)) },
		Var: "--series-1", Area: false,
	}))
	vm.DDSVG = template.HTML(lineChart(vm.DDPts, chartOpts{
		W: 860, H: 160, YFmt: func(v float64) string { return fmtNum(v, 1) + "%" },
		Var: "--neg", Area: true,
	}))

	// pnl mensile
	byMonth := map[string]float64{}
	for _, tr := range r.Trades {
		k := time.UnixMilli(tr.ExitTime).UTC().Format("2006-01")
		byMonth[k] += tr.PnL
	}
	keys := make([]string, 0, len(byMonth))
	for k := range byMonth {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		vm.Monthly = append(vm.Monthly, monthlyPnL{Month: k, PnL: byMonth[k]})
	}
	if len(vm.Monthly) > 24 { // ultime 24 mensilità nel grafico
		vm.Monthly = vm.Monthly[len(vm.Monthly)-24:]
	}
	vm.MonthSVG = template.HTML(barChart(vm.Monthly))

	// righe trade (cronologiche)
	for i, tr := range r.Trades {
		vm.TradeRows = append(vm.TradeRows, tradeRow{
			Idx:      i + 1,
			Entry:    fmtTime(tr.EntryTime),
			Exit:     fmtTime(tr.ExitTime),
			Motive:   tr.Reason,
			EntryPx:  fmtNum(tr.EntryPrice, 2),
			ExitPx:   fmtNum(tr.ExitPrice, 2),
			Size:     fmtNum(tr.InitialSize, 6),
			PnL:      fmtMoney(tr.PnL),
			R:        fmtNum(tr.R, 2),
			PnlClass: pnlClass(tr.PnL),
			Partial:  boolMark(tr.PartialFilled),
		})
	}
	return vm
}

func boolMark(b bool) string {
	if b {
		return "✔"
	}
	return "—"
}

func pnlClass(v float64) string {
	switch {
	case v > 0:
		return "pos"
	case v < 0:
		return "neg"
	}
	return ""
}

func firstT(r backtest.Result) int64 {
	if len(r.EquityTimes) > 0 {
		return r.EquityTimes[0]
	}
	return 0
}

func lastT(r backtest.Result) int64 {
	if len(r.EquityTimes) > 0 {
		return r.EquityTimes[len(r.EquityTimes)-1]
	}
	return 0
}

func last(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	return xs[len(xs)-1]
}

func zip(ts, vs []float64) []point {
	out := make([]point, len(vs))
	for i := range vs {
		out[i] = point{T: ts[i], V: vs[i]}
	}
	return out
}

func downsampleIndices(n, maxPoints int) []int {
	out := make([]int, 0, maxPoints)
	step := float64(n-1) / float64(maxPoints-1)
	for i := 0; i < maxPoints; i++ {
		out = append(out, int(math.Round(float64(i)*step)))
	}
	out[0] = 0
	out[len(out)-1] = n - 1
	return out
}

// WriteFile genera e scrive il report su disco.
func WriteFile(path string, d Data) error {
	html, err := GenerateHTML(d)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(html), 0o644)
}
