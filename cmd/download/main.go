// Command download scarica candele OHLCV da Binance in cache CSV.
//
// Usage:
//
//	go run ./cmd/download -symbol BTCUSDT -interval 5m -days 90
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"quadscalping/internal/data"
	"quadscalping/internal/market"
)

func main() {
	var (
		symbol   = flag.String("symbol", "BTCUSDT", "simbolo (es. BTCUSDT)")
		interval = flag.String("interval", "5m", "timeframe (1m,3m,5m,15m,30m,1h,2h,4h,1d)")
		out      = flag.String("out", "", "file CSV di destinazione (default data/SYMBOL_INTERVAL.csv)")
		days     = flag.Int("days", 0, "giorni di storia (0 = tutto lo storico disponibile)")
	)
	flag.Parse()

	if _, ok := market.TimeframeMinutes[*interval]; !ok {
		log.Fatalf("timeframe non valido: %s", *interval)
	}
	path := *out
	if path == "" {
		path = fmt.Sprintf("data/%s_%s.csv", *symbol, *interval)
	}

	c := data.NewClient()
	ctx := context.Background()

	var start int64
	if *days > 0 {
		tfMin, _ := market.TimeframeMinutes[*interval]
		full := time.Duration(*days*24*60) * time.Minute
		if time.Duration(tfMin)*time.Minute > full {
			full = time.Duration(tfMin) * time.Minute
		}
		start = time.Now().Add(-full).UnixMilli()
	}

	if start > 0 {
		if err := downloadRange(ctx, c, path, *symbol, *interval, start); err != nil {
			log.Fatal(err)
		}
	} else if _, err := data.UpdateCSV(ctx, path, c, *symbol, *interval); err != nil {
		log.Fatal(err)
	}

	cs, err := data.LoadCSV(path)
	if err != nil {
		log.Fatal(err)
	}
	if len(cs) == 0 {
		log.Fatal("nessuna candela scaricata")
	}
	fmt.Printf("✓ %s: %d candele %s (%s → %s) in %s\n",
		*symbol, len(cs), *interval,
		time.UnixMilli(cs[0].Time).UTC().Format("2006-01-02"),
		time.UnixMilli(cs[len(cs)-1].Time).UTC().Format("2006-01-02"),
		path)
}

func downloadRange(ctx context.Context, c *data.Client, path, symbol, interval string, start int64) error {
	cs, err := c.FetchKlines(ctx, symbol, interval, start, 0)
	if err != nil {
		return err
	}
	return data.SaveCSV(path, cs)
}
