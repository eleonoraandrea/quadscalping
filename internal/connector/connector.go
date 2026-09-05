// Package connector definisce il connettore ordini verso i venue di esecuzione.
// "Orderli" = layer di esecuzione ordini: interfaccia Connector con
// implementazioni Paper (simulazione) e Binance (spot, live o testnet).
package connector

import "context"

// Side dell'ordine.
type Side string

// Lati ordine.
const (
	Buy  Side = "BUY"
	Sell Side = "SELL"
)

// OrderRequest è una richiesta di ordine a mercato.
type OrderRequest struct {
	Symbol string
	Side   Side
	Size   float64 // quantità in asset base
	Price  float64 // riferimento informativo (usato dal paper connector)
}

// OrderResult esito dell'esecuzione.
type OrderResult struct {
	OrderID     string
	FilledPrice float64
	FilledSize  float64
	Status      string // FILLED | REJECTED
	Raw         string
}

// Stati ordine.
const (
	StatusFilled   = "FILLED"
	StatusRejected = "REJECTED"
)

// Connector è l'astrazione del venue di esecuzione ordini.
type Connector interface {
	Name() string
	// Balance ritorna il saldo libero dell'asset (es. "USDT").
	Balance(ctx context.Context, asset string) (float64, error)
	// LastPrice ritorna l'ultimo prezzo del simbolo (es. "BTCUSDT").
	LastPrice(ctx context.Context, symbol string) (float64, error)
	// MarketOrder esegue un ordine a mercato.
	MarketOrder(ctx context.Context, req OrderRequest) (OrderResult, error)
}
