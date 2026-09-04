package report

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// escapeAttr rende una stringa sicura dentro un attributo SVG/HTML.
func escapeAttr(s string) string {
	r := strings.NewReplacer("&", "&amp;", "\"", "&quot;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// chartOpts configura un line chart SVG.
type chartOpts struct {
	W, H int
	YFmt func(float64) string
	Var  string // variabile CSS del colore serie (es. --series-1)
	Area bool   // wash al 10% sotto la linea
}

const svgNS = "http://www.w3.org/2000/svg"

// lineChart genera un SVG per una serie temporale: linea 2px,
// griglia hairline, tick puliti, end-dot con anello di superficie.
func lineChart(pts []point, o chartOpts) string {
	if len(pts) < 2 {
		return ""
	}
	const ml, mr, mt, mb = 56, 16, 12, 28
	pw := float64(o.W - ml - mr)
	ph := float64(o.H - mt - mb)

	minV, maxV := math.Inf(1), math.Inf(-1)
	for _, p := range pts {
		minV = math.Min(minV, p.V)
		maxV = math.Max(maxV, p.V)
	}
	ticks := NiceTicks(minV, maxV, 5)
	y0, y1 := ticks[0], ticks[len(ticks)-1]
	span := y1 - y0
	if span <= 0 {
		span = 1
	}
	x := func(i int) float64 {
		return ml + pw*float64(i)/float64(len(pts)-1)
	}
	y := func(v float64) float64 {
		return mt + ph*(1-(v-y0)/span)
	}

	var sb strings.Builder
	// dati hover come attributo: html/template dentro <script> ri-serializza
	// le stringhe, quindi evitiamo il tag script del tutto
	dataJSON := escapeAttr(mustJSON(pts))
	fmt.Fprintf(&sb, `<svg viewBox="0 0 %d %d" class="chart" role="img" aria-label="grafico" preserveAspectRatio="none" data-chart="line" data-points="%s">`,
		o.W, o.H, dataJSON)

	// griglia + tick y
	for _, t := range ticks {
		yy := y(t)
		fmt.Fprintf(&sb, `<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" class="grid"/>`,
			ml, yy, o.W-mr, yy)
		fmt.Fprintf(&sb, `<text x="%d" y="%.1f" class="tick" text-anchor="end">%.9s</text>`,
			ml-8, yy+4, o.YFmt(t))
	}
	// baseline
	fmt.Fprintf(&sb, `<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" class="axis"/>`,
		ml, y(y0), o.W-mr, y(y0))

	// tick x: 5 date
	nLbl := 5
	for k := 0; k < nLbl; k++ {
		i := int(math.Round(float64(len(pts)-1) * float64(k) / float64(nLbl-1)))
		lbl := shortTime(pts[i].T)
		fmt.Fprintf(&sb, `<text x="%.1f" y="%d" class="tick" text-anchor="middle">%s</text>`,
			x(i), o.H-8, lbl)
	}

	// wash area (10%)
	if o.Area {
		var path strings.Builder
		fmt.Fprintf(&path, "M%.2f,%.2f", x(0), y(pts[0].V))
		for i := 1; i < len(pts); i++ {
			fmt.Fprintf(&path, " L%.2f,%.2f", x(i), y(pts[i].V))
		}
		fmt.Fprintf(&path, " L%.2f,%.2f L%.2f,%.2f Z", x(len(pts)-1), y(y0), x(0), y(y0))
		fmt.Fprintf(&sb, `<path d="%s" fill="var(%s)" fill-opacity="0.1"/>`, path.String(), o.Var)
	}

	// linea 2px
	var pl strings.Builder
	for i, p := range pts {
		if i == 0 {
			fmt.Fprintf(&pl, "M%.2f,%.2f", x(i), y(p.V))
		} else {
			fmt.Fprintf(&pl, " L%.2f,%.2f", x(i), y(p.V))
		}
	}
	fmt.Fprintf(&sb, `<path d="%s" fill="none" stroke="var(%s)" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>`,
		pl.String(), o.Var)

	// end-dot con anello superficie
	fmt.Fprintf(&sb, `<circle cx="%.2f" cy="%.2f" r="4" fill="var(%s)" stroke="var(--surface)" stroke-width="2"/>`,
		x(len(pts)-1), y(pts[len(pts)-1].V), o.Var)

	// layer hover (crosshair + dot), mosso da JS
	fmt.Fprintf(&sb, `<line class="cross" x1="0" y1="%d" x2="0" y2="%.1f" style="display:none"/>`, mt, y(y0))
	fmt.Fprintf(&sb, `<circle class="hdot" r="4.5" style="display:none" stroke="var(--surface)" stroke-width="2"/>`)
	fmt.Fprintf(&sb, `<rect class="hitzone" x="%d" y="%d" width="%d" height="%d" fill="transparent"/>`,
		ml, mt, o.W-ml-mr, int(ph))
	sb.WriteString(`</svg>`)
	return sb.String()
}

// barChart genera un column chart diverging (positivo/negativo)
// per il PnL mensile: colonne ≤24px, 4px arrotondati all'estremo dati.
func barChart(months []monthlyPnL) string {
	const w, h = 860, 180
	const ml, mr, mt, mb = 56, 16, 10, 26
	pw := float64(w - ml - mr)
	ph := float64(h - mt - mb)
	n := len(months)
	if n == 0 {
		return ""
	}
	maxAbs := 0.0
	for _, m := range months {
		maxAbs = math.Max(maxAbs, math.Abs(m.PnL))
	}
	if maxAbs <= 0 {
		maxAbs = 1
	}
	ticks := NiceTicks(-maxAbs, maxAbs, 4)
	y0, y1 := ticks[0], ticks[len(ticks)-1]
	span := y1 - y0
	y := func(v float64) float64 { return mt + ph*(1-(v-y0)/span) }
	slot := pw / float64(n)
	bw := math.Min(24, slot*0.6)

	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg viewBox="0 0 %d %d" class="chart" role="img" aria-label="PnL mensile">`, w, h)
	for _, t := range ticks {
		yy := y(t)
		fmt.Fprintf(&sb, `<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" class="grid"/>`, ml, yy, w-mr, yy)
		fmt.Fprintf(&sb, `<text x="%d" y="%.1f" class="tick" text-anchor="end">%s</text>`, ml-8, yy+4, groupInt(int64(t)))
	}
	zero := y(0)
	fmt.Fprintf(&sb, `<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" class="axis"/>`, ml, zero, w-mr, zero)

	for i, m := range months {
		cx := ml + slot*float64(i) + slot/2
		yy := y(m.PnL)
		top, bot := yy, zero
		if m.PnL < 0 {
			top, bot = zero, yy
		}
		height := bot - top
		color := "--series-1"
		if m.PnL < 0 {
			color = "--neg"
		}
		// 4px arrotondati solo all'estremo dati, quadrato alla baseline
		fmt.Fprintf(&sb, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="var(%s)" rx="0" class="bar" data-lbl="%s" data-val="%s"><title>%s: %s</title></rect>`,
			cx-bw/2, top+2, bw, math.Max(height-2, 1), color, m.Month, fmtMoney(m.PnL), m.Month, fmtMoney(m.PnL))
		if n <= 24 || i%(n/12+1) == 0 {
			fmt.Fprintf(&sb, `<text x="%.2f" y="%d" class="tick" text-anchor="middle">%.7s</text>`,
				cx, h-8, m.Month[2:])
		}
	}
	sb.WriteString(`</svg>`)
	return sb.String()
}

func shortTime(ms float64) string {
	return fmtTime(int64(ms))[0:5] // gg-mm
}
