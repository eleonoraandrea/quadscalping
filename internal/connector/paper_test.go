package connector

import (
	"context"
	"math"
	"testing"
)

func fixedPrice(px float64) func(context.Context, string) (float64, error) {
	return func(ctx context.Context, symbol string) (float64, error) { return px, nil }
}

func TestPaperBuyThenSell(t *testing.T) {
	p := NewPaper(10000, 0.001, 0.0005, fixedPrice(100))

	res, err := p.MarketOrder(context.Background(), OrderRequest{
		Symbol: "BTCUSDT", Side: Buy, Size: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusFilled {
		t.Fatalf("status %v", res.Status)
	}
	// fill buy: 100*(1+0.0005) = 100.05 ; cost = 200.1 ; fee = 0.2001
	wantBuy := 100.05
	if math.Abs(res.FilledPrice-wantBuy) > 1e-9 {
		t.Errorf("buy price %v want %v", res.FilledPrice, wantBuy)
	}
	usdt, _ := p.Balance(context.Background(), "USDT")
	wantUSDT := 10000 - 200.1 - 0.2001
	if math.Abs(usdt-wantUSDT) > 1e-9 {
		t.Errorf("usdt %v want %v", usdt, wantUSDT)
	}
	btc, _ := p.Balance(context.Background(), "BTC")
	if math.Abs(btc-2) > 1e-12 {
		t.Errorf("btc %v want 2", btc)
	}

	// vendi tutto: fill 100*(1-0.0005)=99.95, ricavi 199.9, fee 0.1999
	res2, err := p.MarketOrder(context.Background(), OrderRequest{
		Symbol: "BTCUSDT", Side: Sell, Size: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(res2.FilledPrice-99.95) > 1e-9 {
		t.Errorf("sell price %v", res2.FilledPrice)
	}
	usdt2, _ := p.Balance(context.Background(), "USDT")
	want2 := wantUSDT + 199.9 - 0.1999
	if math.Abs(usdt2-want2) > 1e-9 {
		t.Errorf("usdt finale %v want %v", usdt2, want2)
	}
	btc2, _ := p.Balance(context.Background(), "BTC")
	if btc2 != 0 {
		t.Errorf("btc residuo %v", btc2)
	}
}

func TestPaperRejectsInsufficientBalance(t *testing.T) {
	p := NewPaper(100, 0.001, 0, fixedPrice(100))
	res, err := p.MarketOrder(context.Background(), OrderRequest{
		Symbol: "BTCUSDT", Side: Buy, Size: 2, // costa 200 > 100
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusRejected {
		t.Errorf("want REJECTED got %v", res.Status)
	}
}

func TestPaperSellWithoutAssetRejected(t *testing.T) {
	p := NewPaper(1000, 0, 0, fixedPrice(100))
	res, _ := p.MarketOrder(context.Background(), OrderRequest{
		Symbol: "BTCUSDT", Side: Sell, Size: 1,
	})
	if res.Status != StatusRejected {
		t.Errorf("want REJECTED got %v", res.Status)
	}
}

func TestPaperFeeTracking(t *testing.T) {
	p := NewPaper(10000, 0.001, 0, fixedPrice(100))
	p.MarketOrder(context.Background(), OrderRequest{Symbol: "BTCUSDT", Side: Buy, Size: 1})
	if math.Abs(p.FeePaid-0.1) > 1e-9 {
		t.Errorf("feePaid %v want 0.1", p.FeePaid)
	}
}
