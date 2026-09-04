// Command hpsbot esegue il bot live/paper HPS.
//
// Usage:
//
//	go run ./cmd/hpsbot -config config.json
//
// Default: testnet + dry run (nessun ordine reale).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"quadscalping/internal/bot"
	"quadscalping/internal/config"
	"quadscalping/internal/connector"
	"quadscalping/internal/data"
	"quadscalping/internal/market"
)

func main() {
	var cfgPath = flag.String("config", "config.json", "file di configurazione")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if _, ok := market.Minutes(cfg.Trading.Timeframe); !ok {
		log.Fatalf("timeframe non valido: %s", cfg.Trading.Timeframe)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dc := data.NewClient()

	// fetcher candele: ultime 400 barre chiuse dal endpoint pubblico
	fetcher := func(ctx context.Context, symbol string) ([]market.Candle, error) {
		tfMin, _ := market.Minutes(cfg.Trading.Timeframe)
		start := time.Now().Add(-time.Duration(tfMin*420) * time.Minute).UnixMilli()
		return dc.FetchKlines(ctx, symbol, cfg.Trading.Timeframe, start, 0)
	}

	// connettore ordini
	var conn connector.Connector
	if cfg.Exchange.DryRun {
		// dry run: prezzi reali dal ticker pubblico, esecuzione simulata
		priceConn := connector.NewBinance(connector.BinanceConfig{
			Testnet: cfg.Exchange.Testnet,
		})
		conn = connector.NewPaper(cfg.Bot.InitialCapital, cfg.Bot.Commission, cfg.Bot.Slippage,
			func(ctx context.Context, symbol string) (float64, error) {
				return priceConn.LastPrice(ctx, symbol)
			})
		log.Printf("⚠ modalità DRY RUN: ordini simulati su prezzi reali")
	} else {
		if cfg.Exchange.APIKey == "" || cfg.Exchange.APISecret == "" {
			log.Fatalf("ordini reali richiesti BINANCE_API_KEY e BINANCE_API_SECRET")
		}
		conn = connector.NewBinance(connector.BinanceConfig{
			APIKey: cfg.Exchange.APIKey, APISecret: cfg.Exchange.APISecret,
			Testnet:           cfg.Exchange.Testnet,
			QuantityPrecision: cfg.Exchange.QuantityPrecision,
		})
	}

	b, err := bot.New(cfg, conn, fetcher, log.New(os.Stderr, "[hps] ", log.LstdFlags|log.Lmsgprefix))
	if err != nil {
		log.Fatalf("bot: %v", err)
	}

	// status server minimale (health + stato)
	addr := cfg.Bot.HTTPAddr
	if addr != "" {
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"status":"ok"}`)
		})
		mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(b.State())
		})
		go func() {
			srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
			log.Printf("status server su http://localhost%s/status", addr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("status server: %v", err)
			}
		}()
	}

	b.Run(ctx)
}
