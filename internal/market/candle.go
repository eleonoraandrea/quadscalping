// Package market definisce i tipi di dato di mercato condivisi.
package market

// Candle è una candela OHLCV. Time è l'open time in millisecondi Unix (stile Binance).
type Candle struct {
	Time   int64
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// Timeframe supportati (identificatori intervallo Binance).
var TimeframeMinutes = map[string]int{
	"1m":  1,
	"3m":  3,
	"5m":  5,
	"15m": 15,
	"30m": 30,
	"1h":  60,
	"2h":  120,
	"4h":  240,
	"1d":  1440,
}

// Minutes ritorna i minuti del timeframe, ok=false se sconosciuto.
func Minutes(tf string) (int, bool) {
	m, ok := TimeframeMinutes[tf]
	return m, ok
}
