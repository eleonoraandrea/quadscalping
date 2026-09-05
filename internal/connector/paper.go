package connector

import (
	"context"
	"fmt"
	"sync"
)

// PaperConnector simula l'esecuzione: utile per dry-run e test.
type PaperConnector struct {
	mu         sync.Mutex
	Balances   map[string]float64
	Commission float64 // frazione sul controvalore
	Slippage   float64 // frazione sul prezzo
	PriceFn    func(ctx context.Context, symbol string) (float64, error)
	FeePaid    float64
}

// NewPaper crea un paper connector con saldo iniziale in USDT.
func NewPaper(startUSDT, commission, slippage float64, priceFn func(ctx context.Context, symbol string) (float64, error)) *PaperConnector {
	return &PaperConnector{
		Balances:   map[string]float64{"USDT": startUSDT},
		Commission: commission,
		Slippage:   slippage,
		PriceFn:    priceFn,
	}
}

func (p *PaperConnector) Name() string { return "paper" }

func (p *PaperConnector) Balance(ctx context.Context, asset string) (float64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Balances[asset], nil
}

func (p *PaperConnector) LastPrice(ctx context.Context, symbol string) (float64, error) {
	if p.PriceFn == nil {
		return 0, fmt.Errorf("paper: PriceFn non impostata")
	}
	return p.PriceFn(ctx, symbol)
}

func (p *PaperConnector) MarketOrder(ctx context.Context, req OrderRequest) (OrderResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	px := req.Price
	if px <= 0 && p.PriceFn != nil {
		var err error
		px, err = p.PriceFn(ctx, req.Symbol)
		if err != nil {
			return OrderResult{Status: StatusRejected}, err
		}
	}
	if px <= 0 {
		return OrderResult{Status: StatusRejected}, fmt.Errorf("paper: prezzo non disponibile")
	}

	base, quote := splitSymbol(req.Symbol)

	switch req.Side {
	case Buy:
		fill := px * (1 + p.Slippage)
		cost := fill * req.Size
		fee := cost * p.Commission
		if p.Balances[quote] < cost+fee {
			return OrderResult{Status: StatusRejected, Raw: fmt.Sprintf(
				"saldo %s insufficiente (%.8f < %.8f)", quote, p.Balances[quote], cost+fee)}, nil
		}
		p.Balances[quote] -= cost + fee
		p.Balances[base] += req.Size
		p.FeePaid += fee
		return OrderResult{FilledPrice: fill, FilledSize: req.Size, Status: StatusFilled}, nil

	case Sell:
		if p.Balances[base] < req.Size {
			return OrderResult{Status: StatusRejected, Raw: fmt.Sprintf(
				"saldo %s insufficiente (%.8f < %.8f)", base, p.Balances[base], req.Size)}, nil
		}
		fill := px * (1 - p.Slippage)
		proceeds := fill * req.Size
		fee := proceeds * p.Commission
		p.Balances[base] -= req.Size
		p.Balances[quote] += proceeds - fee
		p.FeePaid += fee
		return OrderResult{FilledPrice: fill, FilledSize: req.Size, Status: StatusFilled}, nil

	default:
		return OrderResult{Status: StatusRejected}, fmt.Errorf("paper: side non valido %q", req.Side)
	}
}

// splitSymbol divide "BTCUSDT" in base "BTC" e quote "USDT".
// euristica: quote = suffisso noto più lungo.
func splitSymbol(symbol string) (base, quote string) {
	for _, q := range []string{"USDT", "USDC", "BUSD", "FDUSD", "BTC", "ETH", "EUR"} {
		if len(symbol) > len(q) && symbol[len(symbol)-len(q):] == q {
			return symbol[:len(symbol)-len(q)], q
		}
	}
	return symbol, "USDT"
}
