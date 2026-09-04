package bot

import (
	"context"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"quadscalping/internal/config"
	"quadscalping/internal/connector"
	"quadscalping/internal/market"
)

// serie sintetica: discesa con pop alla barra indicata (come backtest_test)
func synth(n, popAt int) []market.Candle {
	cs := make([]market.Candle, n)
	for i := 0; i < n; i++ {
		c := 300 - 0.5*float64(i)
		cs[i] = market.Candle{
			Time: int64(i) * 300000, Open: c + 0.25,
			High: c + 0.2, Low: c - 0.2, Close: c, Volume: 100,
		}
	}
	if popAt >= 0 && popAt < n {
		c := cs[popAt].Close + 1.0
		cs[popAt].Close = c
		cs[popAt].High = c + 0.2
		cs[popAt].Open = c
	}
	return cs
}

// growingFeed espone un prefisso crescente di una serie lunga:
// ogni fetch ritorna le prime window barre.
type growingFeed struct {
	mu     sync.Mutex
	series []market.Candle
	window int
	// lastClose aggiornato a ogni fetch, usato dal priceFn del paper connector
	lastClose float64
}

func (g *growingFeed) fetch(_ context.Context, _ string) ([]market.Candle, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	end := g.window
	if end > len(g.series) {
		end = len(g.series)
	}
	f := g.series[:end]
	if len(f) > 0 {
		g.lastClose = f[len(f)-1].Close
	}
	return f, nil
}

func (g *growingFeed) price(_ context.Context, _ string) (float64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lastClose, nil
}

// advance espone una barra in più.
func (g *growingFeed) advance(n int) {
	g.mu.Lock()
	g.window += n
	g.mu.Unlock()
}

// setPrice forza il prezzo corrente (per simulare movimenti tra i cicli).
func (g *growingFeed) setPrice(px float64) {
	g.mu.Lock()
	g.lastClose = px
	g.mu.Unlock()
}

func (g *growingFeed) priceNow() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lastClose
}

func testConfig(t *testing.T) config.Config {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Bot.StateFile = filepath.Join(dir, "state.json")
	cfg.Bot.PollIntervalSec = 1
	return cfg
}

// bigSeries: 700 barre, pop all'indice 299 (l'ultima del primo frame da 300).
func bigSeries() []market.Candle { return synth(700, 299) }

func TestBotOpensAndStopsOut(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()

	feed := &growingFeed{series: bigSeries(), window: 300}
	conn := connector.NewPaper(10000, 0.0004, 0.0001, feed.price)

	b, err := New(cfg, conn, feed.fetch, log.New(os.Stderr, "", 0))
	if err != nil {
		t.Fatal(err)
	}

	b.Cycle(ctx) // frame [0..299], pop sull'ultima barra -> entry
	st := b.State()
	pos, ok := st.Positions["BTCUSDT"]
	if !ok {
		t.Fatal("posizione non aperta dopo il ciclo di entry")
	}
	if pos.EntryPrice <= 0 || pos.Stop >= pos.EntryPrice || pos.TP1 <= pos.EntryPrice {
		t.Errorf("parametri posizione incoerenti: %+v", pos)
	}

	// la discesa prosegue barra per barra: lo stop deve scattare
	for i := 0; i < 6 && len(b.State().Positions) > 0; i++ {
		feed.advance(1)
		b.Cycle(ctx)
	}
	st = b.State()
	if len(st.Positions) != 0 {
		t.Fatalf("posizione non chiusa: %+v", st.Positions)
	}
	if st.ClosedTrades != 1 {
		t.Errorf("closed trades %d want 1", st.ClosedTrades)
	}
	if st.RealizedPnL >= 0 {
		t.Errorf("stop loss deve dare pnl negativo: %v", st.RealizedPnL)
	}
	usdt, _ := conn.Balance(ctx, "USDT")
	btc, _ := conn.Balance(ctx, "BTC")
	if btc != 0 {
		t.Errorf("base residua %v", btc)
	}
	if math.Abs(usdt-(10000+st.RealizedPnL)) > 0.01 {
		t.Errorf("usdt %v vs 10000+pnl %v", usdt, 10000+st.RealizedPnL)
	}
}

func TestBotStatePersists(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()

	feed := &growingFeed{series: bigSeries(), window: 300}
	conn := connector.NewPaper(10000, 0.0004, 0, feed.price)
	b1, err := New(cfg, conn, feed.fetch, log.New(os.Stderr, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	b1.Cycle(ctx)
	if len(b1.State().Positions) != 1 {
		t.Fatal("setup: posizione attesa")
	}

	// nuovo bot sulla stessa state file deve ritrovare la posizione
	b2, err := New(cfg, conn, feed.fetch, log.New(os.Stderr, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(b2.State().Positions) != 1 {
		t.Fatalf("posizione non ripristinata: %+v", b2.State().Positions)
	}
}

func TestBotDailyLossLimitBlocksEntries(t *testing.T) {
	cfg := testConfig(t)
	cfg.Trading.MaxDailyLossPct = 0.0001 // ~1$ su 10000
	ctx := context.Background()

	feed := &growingFeed{series: bigSeries(), window: 300}
	conn := connector.NewPaper(10000, 0.0004, 0, feed.price)
	b, err := New(cfg, conn, feed.fetch, log.New(os.Stderr, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	b.Cycle(ctx) // entry
	if len(b.State().Positions) != 1 {
		t.Fatal("setup: entry attesa")
	}
	// chiusura forzata in perdita: il prezzo di mercato scende sotto l'entry
	feed.setPrice(feed.priceNow() - 5)
	b.closePosition(ctx, "BTCUSDT", feed.priceNow(), "MANUAL")
	if len(b.State().Positions) != 0 {
		t.Fatal("setup: chiusura attesa")
	}
	if b.State().RealizedPnL >= 0 {
		t.Fatalf("setup: perdita attesa, got %v", b.State().RealizedPnL)
	}

	// la stessa condizione di entry si ripresenta ma il limite deve bloccare
	b.Cycle(ctx)
	if len(b.State().Positions) != 0 {
		t.Error("entry dopo limite giornaliero deve essere bloccata")
	}
}
