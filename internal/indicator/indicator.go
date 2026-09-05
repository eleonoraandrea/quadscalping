// Package indicator implementa gli indicatori tecnici usati da HPS.
// Tutte le serie ritornate hanno stessa lunghezza dell'input;
// i periodi di warmup sono riempiti con math.NaN().
package indicator

import "math"

func isNaN(x float64) bool { return math.IsNaN(x) }

// SMA ritorna la media mobile semplice a periodo n.
// I NaN in testa alla serie (warmup di indicatori a monte) sono saltati:
// la finestra parte dal primo valore valido.
func SMA(xs []float64, n int) []float64 {
	out := make([]float64, len(xs))
	if n <= 0 {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}
	start := -1
	for i, x := range xs {
		if !isNaN(x) {
			start = i
			break
		}
	}
	if start < 0 {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}
	tail := smaRaw(xs[start:], n)
	for i := 0; i < start; i++ {
		out[i] = math.NaN()
	}
	copy(out[start:], tail)
	return out
}

// smaRaw calcola la SMA su una serie senza NaN in testa.
func smaRaw(xs []float64, n int) []float64 {
	out := make([]float64, len(xs))
	if n <= 0 {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}
	var sum float64
	for i, x := range xs {
		sum += x
		if i >= n {
			sum -= xs[i-n]
		}
		if i >= n-1 {
			out[i] = sum / float64(n)
		} else {
			out[i] = math.NaN()
		}
	}
	return out
}

// EMA ritorna la media mobile esponenziale a periodo p,
// inizializzata con la SMA dei primi p valori.
func EMA(xs []float64, p int) []float64 {
	out := make([]float64, len(xs))
	if p <= 0 || len(xs) == 0 {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}
	alpha := 2.0 / float64(p+1)
	var sum float64
	var prev = math.NaN()
	for i, x := range xs {
		if isNaN(x) {
			out[i] = math.NaN()
			continue
		}
		if i < p-1 {
			sum += x
			out[i] = math.NaN()
			continue
		}
		if i == p-1 {
			sum += x
			prev = sum / float64(p)
		} else {
			prev = alpha*x + (1-alpha)*prev
		}
		out[i] = prev
	}
	return out
}

// StochK ritorna la %K dello Stochastic a periodo n: 100*(C-LL)/(HH-LL).
// Finestra piatta (HH==LL) ritorna 50 (neutro).
func StochK(highs, lows, closes []float64, n int) []float64 {
	out := make([]float64, len(closes))
	for i := range out {
		if i < n-1 {
			out[i] = math.NaN()
			continue
		}
		hh, ll := highs[i], lows[i]
		for j := i - n + 1; j <= i; j++ {
			if highs[j] > hh {
				hh = highs[j]
			}
			if lows[j] < ll {
				ll = lows[j]
			}
		}
		if hh == ll {
			out[i] = 50
			continue
		}
		out[i] = 100 * (closes[i] - ll) / (hh - ll)
	}
	return out
}

// StochD ritorna la %D: SMA della %K a periodo d.
// Salta i NaN della %K (il warmup della D parte dalla prima K valida).
func StochD(k []float64, d int) []float64 {
	// trova primo indice valido
	start := -1
	for i, x := range k {
		if !isNaN(x) {
			start = i
			break
		}
	}
	if start < 0 {
		out := make([]float64, len(k))
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}
	tail := SMA(k[start:], d)
	out := make([]float64, len(k))
	for i := 0; i < start; i++ {
		out[i] = math.NaN()
	}
	copy(out[start:], tail)
	return out
}

// TrueRange ritorna il true range per candele gi� pronte (indice 0 = NaN).
func TrueRange(highs, lows, closes []float64) []float64 {
	out := make([]float64, len(closes))
	out[0] = math.NaN()
	for i := 1; i < len(closes); i++ {
		tr := highs[i] - lows[i]
		if v := math.Abs(highs[i] - closes[i-1]); v > tr {
			tr = v
		}
		if v := math.Abs(lows[i] - closes[i-1]); v > tr {
			tr = v
		}
		out[i] = tr
	}
	return out
}

// ATR ritorna l'Average True Range a periodo n (SMA del TR).
func ATR(highs, lows, closes []float64, n int) []float64 {
	tr := TrueRange(highs, lows, closes)
	// TR[0] � NaN: shift di 1 come in StochD
	return StochD(tr, n)
}

// RollingVWAP ritorna il VWAP su finestra mobile di n candele.
// TP = (H+L+C)/3. Se il volume della finestra � 0, fallback sulla SMA del TP.
func RollingVWAP(highs, lows, closes, volumes []float64, n int) []float64 {
	out := make([]float64, len(closes))
	var sumPV, sumV float64
	tp := make([]float64, len(closes))
	for i := range closes {
		tp[i] = (highs[i] + lows[i] + closes[i]) / 3
	}
	tpSMA := SMA(tp, n)
	for i := range closes {
		sumPV += tp[i] * volumes[i]
		sumV += volumes[i]
		if i >= n {
			sumPV -= tp[i-n] * volumes[i-n]
			sumV -= volumes[i-n]
		}
		if i < n-1 {
			out[i] = math.NaN()
			continue
		}
		if sumV <= 0 {
			out[i] = tpSMA[i]
			continue
		}
		out[i] = sumPV / sumV
	}
	return out
}
