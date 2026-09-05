package connector

import (
	"context"
	"encoding/hex"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

// vettore noto HMAC-SHA256
func TestSignHMAC(t *testing.T) {
	sig := SignHMAC([]byte("key"), "The quick brown fox jumps over the lazy dog")
	want := "f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8"
	if hex.EncodeToString(sig) != want {
		t.Errorf("sig %x want %s", sig, want)
	}
}

func TestRoundQuantityFloors(t *testing.T) {
	cases := []struct {
		in   float64
		prec int
		want float64
	}{
		{0.123456789, 5, 0.12345},
		{1.999999, 2, 1.99},
		{2.0, 3, 2.0},
	}
	for _, c := range cases {
		if got := RoundQuantity(c.in, c.prec); math.Abs(got-c.want) > 1e-12 {
			t.Errorf("RoundQuantity(%v,%d)=%v want %v", c.in, c.prec, got, c.want)
		}
	}
}

func TestBinanceDryRunSkipsHTTP(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	bc := NewBinance(BinanceConfig{
		APIKey: "k", APISecret: "s", BaseURL: srv.URL,
		DryRun: true, QuantityPrecision: 6,
	})
	res, err := bc.MarketOrder(context.Background(), OrderRequest{
		Symbol: "BTCUSDT", Side: Buy, Size: 0.123456789, Price: 50000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("dry run non deve chiamare HTTP")
	}
	if res.Status != StatusFilled {
		t.Errorf("status %v", res.Status)
	}
	if math.Abs(res.FilledSize-0.123456) > 1e-12 {
		t.Errorf("size %v want arrotondata a 0.123456", res.FilledSize)
	}
	if res.FilledPrice != 50000 {
		t.Errorf("price %v", res.FilledPrice)
	}
}

func TestBinanceAccountBalance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/account" {
			t.Errorf("path %v", r.URL.Path)
		}
		if r.Header.Get("X-MBX-APIKEY") != "mykey" {
			t.Errorf("api key header mancante")
		}
		if r.URL.Query().Get("signature") == "" {
			t.Errorf("firma mancante")
		}
		w.Write([]byte(`{"balances":[
			{"asset":"BTC","free":"0.5","locked":"0.1"},
			{"asset":"USDT","free":"1234.56","locked":"0"}]}`))
	}))
	defer srv.Close()

	bc := NewBinance(BinanceConfig{APIKey: "mykey", APISecret: "s", BaseURL: srv.URL})
	usdt, err := bc.Balance(context.Background(), "USDT")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(usdt-1234.56) > 1e-9 {
		t.Errorf("usdt %v", usdt)
	}
	btc, _ := bc.Balance(context.Background(), "BTC")
	if math.Abs(btc-0.5) > 1e-9 {
		t.Errorf("btc %v (want solo free)", btc)
	}
}

func TestBinanceLastPrice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/ticker/price" {
			t.Errorf("path %v", r.URL.Path)
		}
		if r.URL.Query().Get("symbol") != "BTCUSDT" {
			t.Errorf("symbol %v", r.URL.Query().Get("symbol"))
		}
		w.Write([]byte(`{"symbol":"BTCUSDT","price":"43210.50"}`))
	}))
	defer srv.Close()

	bc := NewBinance(BinanceConfig{BaseURL: srv.URL})
	px, err := bc.LastPrice(context.Background(), "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(px-43210.50) > 1e-9 {
		t.Errorf("price %v", px)
	}
}

func TestBinanceMarketOrderPostsSignedForm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %v", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		q := r.PostForm
		if q.Get("symbol") != "BTCUSDT" || q.Get("side") != "BUY" ||
			q.Get("type") != "MARKET" || q.Get("quantity") == "" {
			t.Errorf("params ordine sbagliati: %v", q)
		}
		if q.Get("signature") == "" || q.Get("timestamp") == "" {
			t.Errorf("firma/timestamp mancanti")
		}
		w.Write([]byte(`{"symbol":"BTCUSDT","orderId":28,` +
			`"executedQty":"0.50000000","cummulativeQuoteQty":"25000.00","status":"FILLED"}`))
	}))
	defer srv.Close()

	bc := NewBinance(BinanceConfig{APIKey: "k", APISecret: "s",
		BaseURL: srv.URL, QuantityPrecision: 6})
	res, err := bc.MarketOrder(context.Background(), OrderRequest{
		Symbol: "BTCUSDT", Side: Buy, Size: 0.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusFilled || res.OrderID != "28" {
		t.Errorf("res %+v", res)
	}
	if math.Abs(res.FilledPrice-50000) > 1e-6 { // 25000/0.5
		t.Errorf("avg fill price %v want 50000", res.FilledPrice)
	}
}
