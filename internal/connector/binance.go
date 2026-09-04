package connector

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// BinanceConfig configura il connettore Binance spot.
type BinanceConfig struct {
	APIKey, APISecret string
	Testnet           bool   // usa https://testnet.binance.vision
	BaseURL           string // override esplicito (usato nei test)
	QuantityPrecision int    // decimali della quantità (default 6)
	DryRun            bool   // se true non invia ordini reali
	RecvWindowMs      int64  // default 5000
}

// BinanceConnector esegue ordini market su Binance spot (o testnet).
type BinanceConnector struct {
	cfg BinanceConfig
	hc  *http.Client
}

// NewBinance crea il connettore.
func NewBinance(cfg BinanceConfig) *BinanceConnector {
	if cfg.BaseURL == "" {
		if cfg.Testnet {
			cfg.BaseURL = "https://testnet.binance.vision"
		} else {
			cfg.BaseURL = "https://api.binance.com"
		}
	}
	if cfg.QuantityPrecision <= 0 {
		cfg.QuantityPrecision = 6
	}
	if cfg.RecvWindowMs <= 0 {
		cfg.RecvWindowMs = 5000
	}
	return &BinanceConnector{cfg: cfg, hc: &http.Client{Timeout: 30 * time.Second}}
}

func (b *BinanceConnector) Name() string {
	switch {
	case b.cfg.DryRun:
		return "binance-dryrun"
	case b.cfg.Testnet:
		return "binance-testnet"
	default:
		return "binance"
	}
}

// SignHMAC calcola HMAC-SHA256(secret, msg).
func SignHMAC(secret []byte, msg string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(msg))
	return m.Sum(nil)
}

// RoundQuantity tronca la quantità alla precisione del venue (floor).
func RoundQuantity(v float64, precision int) float64 {
	p := math.Pow10(precision)
	return math.Floor(v*p) / p
}

// signedRequest esegue una richiesta firmata HMAC.
func (b *BinanceConnector) signedRequest(ctx context.Context, method, path string, params url.Values) ([]byte, error) {
	if b.cfg.APIKey == "" {
		return nil, fmt.Errorf("binance: API key non configurata")
	}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Set("recvWindow", strconv.FormatInt(b.cfg.RecvWindowMs, 10))
	sig := hex.EncodeToString(SignHMAC([]byte(b.cfg.APISecret), params.Encode()))
	params.Set("signature", sig)

	var body io.Reader
	u := b.cfg.BaseURL + path
	if method == http.MethodPost {
		body = io.Reader(strings.NewReader(params.Encode()))
	} else {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MBX-APIKEY", b.cfg.APIKey)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := b.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance %s %s: %s", method, resp.Status, truncateStr(data, 300))
	}
	return data, nil
}

func truncateStr(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n])
	}
	return string(b)
}

func (b *BinanceConnector) publicGet(ctx context.Context, path string, params url.Values) ([]byte, error) {
	u := b.cfg.BaseURL + path + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance %s: %s", resp.Status, truncateStr(data, 300))
	}
	return data, nil
}

// Balance ritorna il saldo libero dell'asset.
func (b *BinanceConnector) Balance(ctx context.Context, asset string) (float64, error) {
	data, err := b.signedRequest(ctx, http.MethodGet, "/api/v3/account", url.Values{})
	if err != nil {
		return 0, err
	}
	var acc struct {
		Balances []struct {
			Asset string  `json:"asset"`
			Free  float64 `json:"free,string"`
		} `json:"balances"`
	}
	if err := json.Unmarshal(data, &acc); err != nil {
		return 0, fmt.Errorf("binance: account non valido: %w", err)
	}
	for _, bal := range acc.Balances {
		if bal.Asset == asset {
			return bal.Free, nil
		}
	}
	return 0, nil
}

// LastPrice ritorna l'ultimo prezzo dal ticker pubblico.
func (b *BinanceConnector) LastPrice(ctx context.Context, symbol string) (float64, error) {
	data, err := b.publicGet(ctx, "/api/v3/ticker/price", url.Values{"symbol": {symbol}})
	if err != nil {
		return 0, err
	}
	var out struct {
		Price float64 `json:"price,string"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return 0, fmt.Errorf("binance: ticker non valido: %w", err)
	}
	return out.Price, nil
}

// MarketOrder invia un ordine market. Con DryRun simula il fill
// al prezzo di riferimento senza chiamare l'exchange.
func (b *BinanceConnector) MarketOrder(ctx context.Context, req OrderRequest) (OrderResult, error) {
	qty := RoundQuantity(req.Size, b.cfg.QuantityPrecision)
	if qty <= 0 {
		return OrderResult{Status: StatusRejected, Raw: "quantità nulla dopo arrotondamento"}, nil
	}

	if b.cfg.DryRun {
		return OrderResult{
			FilledPrice: req.Price, FilledSize: qty, Status: StatusFilled,
			Raw: "DRY_RUN", OrderID: "dryrun",
		}, nil
	}

	params := url.Values{}
	params.Set("symbol", req.Symbol)
	params.Set("side", string(req.Side))
	params.Set("type", "MARKET")
	params.Set("quantity", strconv.FormatFloat(qty, 'f', -1, 64))

	data, err := b.signedRequest(ctx, http.MethodPost, "/api/v3/order", params)
	if err != nil {
		return OrderResult{Status: StatusRejected}, err
	}
	var o struct {
		OrderID     int64   `json:"orderId"`
		ExecutedQty float64 `json:"executedQty,string"`
		CumQuoteQty float64 `json:"cummulativeQuoteQty,string"`
		Status      string  `json:"status"`
	}
	if err := json.Unmarshal(data, &o); err != nil {
		return OrderResult{Status: StatusRejected, Raw: string(data)}, err
	}
	avg := 0.0
	if o.ExecutedQty > 0 {
		avg = o.CumQuoteQty / o.ExecutedQty
	}
	return OrderResult{
		OrderID:     strconv.FormatInt(o.OrderID, 10),
		FilledPrice: avg,
		FilledSize:  o.ExecutedQty,
		Status:      o.Status,
		Raw:         string(data),
	}, nil
}
