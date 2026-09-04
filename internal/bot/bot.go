// Package bot è l'orchestratore live/paper del sistema HPS.
package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"sync"
	"time"

	"quadscalping/internal/indicator"

	"quadscalping/internal/config"
	"quadscalping/internal/connector"
	"quadscalping/internal/market"
	"quadscalping/internal/risk"
	"quadscalping/internal/strategy"
	"quadscalping/internal/telegram"
)

// Position è una posizione aperta tracciata dal bot.
type Position struct {
	Symbol      string  `json:"symbol"`
	EntryPrice  float64 `json:"entry_price"`
	Stop        float64 `json:"stop"`
	TP1         float64 `json:"tp1"`
	TP2         float64 `json:"tp2"`
	Size        float64 `json:"size"`
	InitialSize float64 `json:"initial_size"`
	RiskAmount  float64 `json:"risk_amount"`
	EntryTime   int64   `json:"entry_time"`
	Partial     bool    `json:"partial"`
	Strength    float64 `json:"strength"`
	OpenFee     float64 `json:"open_fee"`
	HighSince   float64 `json:"high_since"` // massimo dall'entry (trailing)
}

// State è lo stato persistito del bot.
type State struct {
	Positions    map[string]*Position `json:"positions"`
	DailyPnL     float64              `json:"daily_pnl"`
	DailyTrades  int                  `json:"daily_trades"`
	RealizedPnL  float64              `json:"realized_pnl"`
	ClosedTrades int                  `json:"closed_trades"`
	PeakEquity   float64              `json:"peak_equity"`
	ConsecLosses int                  `json:"consec_losses"`
	PauseUntilMs int64                `json:"pause_until_ms"`
	UpdatedAt    string               `json:"updated_at"`
}

// Bot è l'orchestratore.
type Bot struct {
	cfg     config.Config
	params  map[string]strategy.Params // per simbolo (symbol_overrides)
	risks   map[string]config.Risk     // per simbolo (risk_overrides)
	conn    connector.Connector
	risk    *risk.Manager
	tg      *telegram.Notifier
	mu      sync.Mutex
	state   State
	fetcher func(ctx context.Context, symbol string) ([]market.Candle, error)
	log     *log.Logger
}

// Fetcher fornisce le candele (inietabile per i test).
type Fetcher func(ctx context.Context, symbol string) ([]market.Candle, error)

// New crea il bot.
func New(cfg config.Config, conn connector.Connector, fetcher Fetcher, logger *log.Logger) (*Bot, error) {
	if fetcher == nil {
		return nil, fmt.Errorf("bot: fetcher richiesto")
	}
	b := &Bot{
		cfg:     cfg,
		params:  map[string]strategy.Params{},
		risks:   map[string]config.Risk{},
		conn:    conn,
		risk:    risk.New(cfg.Trading.RiskPct, cfg.Trading.MaxDailyLossPct, cfg.Trading.MaxPositions),
		tg:      telegram.New(cfg.Telegram.BotToken, cfg.Telegram.ChatID, cfg.Telegram.Enabled),
		fetcher: fetcher,
		log:     logger,
	}
	for _, s := range cfg.Trading.Symbols {
		b.params[s] = cfg.StrategyParamsFor(s)
		b.risks[s] = cfg.RiskConfigFor(s)
	}
	b.state.Positions = map[string]*Position{}
	if b.state.PeakEquity <= 0 {
		b.state.PeakEquity = cfg.Bot.InitialCapital
	}
	if b.log == nil {
		b.log = log.New(os.Stderr, "[hps] ", log.LstdFlags|log.Lmsgprefix)
	}
	if err := b.loadState(); err != nil {
		return nil, err
	}
	return b, nil
}

// State ritorna una copia dello stato corrente.
func (b *Bot) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := b.state
	st.Positions = make(map[string]*Position, len(b.state.Positions))
	for k, v := range b.state.Positions {
		cp := *v
		st.Positions[k] = &cp
	}
	return st
}

func (b *Bot) loadState() error {
	data, err := os.ReadFile(b.cfg.Bot.StateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return fmt.Errorf("stato corrotto %s: %w", b.cfg.Bot.StateFile, err)
	}
	if st.Positions != nil {
		b.state = st
	}
	return nil
}

func (b *Bot) saveStateLocked() {
	if eq := b.cfg.Bot.InitialCapital + b.state.RealizedPnL; eq > b.state.PeakEquity {
		b.state.PeakEquity = eq
	}
	b.state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(b.state, "", "  ")
	if err != nil {
		b.log.Printf("errore marshal stato: %v", err)
		return
	}
	if err := os.WriteFile(b.cfg.Bot.StateFile, data, 0o644); err != nil {
		b.log.Printf("errore salvataggio stato: %v", err)
	}
}

// Run esegue il loop principale finché ctx non viene cancellato.
func (b *Bot) Run(ctx context.Context) {
	interval := time.Duration(b.cfg.Bot.PollIntervalSec) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}
	b.log.Printf("avviato: connettore=%s simboli=%v tf=%s posizioni aperte=%d",
		b.conn.Name(), b.cfg.Trading.Symbols, b.cfg.Trading.Timeframe, len(b.state.Positions))
	_ = b.tg.Send(ctx, fmt.Sprintf("🚀 <b>HPS Bot avviato</b>\n🔧 connettore: %s\n📈 %v @ %s",
		b.conn.Name(), b.cfg.Trading.Symbols, b.cfg.Trading.Timeframe))

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	b.Cycle(ctx)
	for {
		select {
		case <-ctx.Done():
			b.log.Printf("arresto (%v)", ctx.Err())
			_ = b.tg.Send(ctx, "⏹️ HPS Bot fermato")
			return
		case <-ticker.C:
			b.Cycle(ctx)
		}
	}
}

// Cycle esegue un giro di scansione/gestione.
func (b *Bot) Cycle(ctx context.Context) {
	symbols := append([]string{}, b.cfg.Trading.Symbols...)
	// prima gestisci le posizioni aperte, poi cerca nuovi segnali
	for _, s := range symbols {
		if _, open := b.state.Positions[s]; open {
			b.managePosition(ctx, s)
		}
	}
	for _, s := range symbols {
		if _, open := b.state.Positions[s]; open {
			continue
		}
		b.scanSymbol(ctx, s)
	}
}

func (b *Bot) candles(ctx context.Context, symbol string) []market.Candle {
	cs, err := b.fetcher(ctx, symbol)
	if err != nil {
		b.log.Printf("fetch %s: %v", symbol, err)
		return nil
	}
	p := b.params[symbol]
	if len(cs) < strategy.WarmupBars(p)+2 {
		return nil
	}
	return cs
}

func (b *Bot) scanSymbol(ctx context.Context, symbol string) {
	if time.Now().UnixMilli() < b.State().PauseUntilMs {
		return // pausa dopo streak di perdite
	}
	cs := b.candles(ctx, symbol)
	if cs == nil {
		return
	}
	p := b.params[symbol]
	ser := strategy.Compute(cs, p)
	sig := ser.EvaluateLast(cs, p)
	if sig.Type != strategy.BuyEntry {
		return
	}
	b.openPosition(ctx, symbol, cs, sig, &ser)
}

func (b *Bot) openPosition(ctx context.Context, symbol string, cs []market.Candle, sig strategy.Signal, ser *strategy.Series) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	ok, reason := b.risk.CanOpen(now, b.equityUnlocked(ctx), len(b.state.Positions))
	if !ok {
		b.log.Printf("risk manager blocca %s: %s", symbol, reason)
		return
	}

	eq := b.equityUnlocked(ctx)
	px, _ := b.conn.LastPrice(ctx, symbol)
	if px <= 0 {
		px = cs[len(cs)-1].Close
	}
	entry := px
	if entry <= sig.StopPrice {
		b.log.Printf("entry %s scartata: prezzo %.2f non sopra lo stop %.2f",
			symbol, entry, sig.StopPrice)
		return
	}
	size := b.risk.PositionSize(eq, entry, sig.StopPrice)
	size *= b.mmFactor(ctx, symbol, sig, ser, eq)
	if size <= 0 {
		return
	}
	// cap leva 1x
	if max := eq / entry; size > max {
		size = max
	}

	res, err := b.conn.MarketOrder(ctx, connector.OrderRequest{
		Symbol: symbol, Side: connector.Buy, Size: size, Price: entry,
	})
	if err != nil || res.Status != connector.StatusFilled {
		b.log.Printf("ordine BUY %s rifiutato: err=%v res=%+v", symbol, err, res)
		return
	}
	fill := res.FilledPrice
	if fill <= 0 {
		fill = entry
	}
	riskUnit := fill - sig.StopPrice
	if riskUnit <= 0 {
		b.log.Printf("ENTRY %s: fill %.2f sotto lo stop %.2f, anomalia", symbol, fill, sig.StopPrice)
		return
	}
	pos := &Position{
		Symbol:      symbol,
		EntryPrice:  fill,
		Stop:        sig.StopPrice,
		TP1:         fill + riskUnit*b.params[symbol].TP1R,
		TP2:         sig.TP2,
		Size:        res.FilledSize,
		InitialSize: res.FilledSize,
		RiskAmount:  riskUnit * res.FilledSize,
		EntryTime:   cs[len(cs)-1].Time,
		Strength:    sig.Strength,
		OpenFee:     fill * res.FilledSize * b.cfg.Bot.Commission,
		HighSince:   fill,
	}
	b.state.Positions[symbol] = pos
	b.saveStateLocked()
	b.log.Printf("ENTRY %s @ %.2f size %.6f stop %.2f tp1 %.2f (strength %.0f)",
		symbol, fill, res.FilledSize, pos.Stop, pos.TP1, sig.Strength)
	_ = b.tg.Send(ctx, fmt.Sprintf("🎯 <b>ENTRY</b> %s @ %.2f\n📦 %.6f\n🛑 %.2f\n🎯 %.2f\n💪 %.0f/100",
		symbol, fill, res.FilledSize, pos.Stop, pos.TP1, sig.Strength))
}

// mmFactor calcola il moltiplicatore di money management per l'entry.
func (b *Bot) mmFactor(ctx context.Context, symbol string, sig strategy.Signal, ser *strategy.Series, eq float64) float64 {
	r := b.risks[symbol]
	if r == (config.Risk{}) {
		r = b.cfg.Risk
	}
	f := 1.0
	last := len(ser.ATR) - 1
	if r.VolAdjust && last >= 0 && ser.ATR[last] > 0 {
		lb := r.VolLookback
		if lb <= 0 {
			lb = 100
		}
		volAvg := indicator.SMA(ser.ATR, lb)
		if v := volAvg[last]; !math.IsNaN(v) && v > 0 {
			x := v / ser.ATR[last]
			f *= math.Max(0.5, math.Min(1.25, x))
		}
	}
	if r.StrengthSizing {
		f *= 0.5 + 0.5*sig.Strength/100
	}
	if r.DDThrottlePct > 0 && b.state.PeakEquity > 0 && eq > 0 {
		if dd := (b.state.PeakEquity - eq) / b.state.PeakEquity; dd > 0 {
			f *= math.Max(0.25, math.Min(1, 1-dd/r.DDThrottlePct))
		}
	}
	return f
}

func (b *Bot) equityUnlocked(ctx context.Context) float64 {
	eq, err := b.conn.Balance(ctx, "USDT")
	if err != nil || eq <= 0 {
		return b.cfg.Bot.InitialCapital
	}
	return eq
}

func (b *Bot) managePosition(ctx context.Context, symbol string) {
	b.mu.Lock()
	pos, ok := b.state.Positions[symbol]
	if !ok {
		b.mu.Unlock()
		return
	}

	cs := b.candles(ctx, symbol)
	if cs == nil {
		b.mu.Unlock()
		return
	}
	last := cs[len(cs)-1]
	p := b.params[symbol]
	// Rilascia il lock prima di computazioni lunghe
	b.mu.Unlock()

	ser := strategy.Compute(cs, p)
	sig := ser.EvaluateLast(cs, p)

	// Riacquisisci il lock per leggere/aggiornare la posizione
	b.mu.Lock()
	pos, ok = b.state.Positions[symbol]
	if !ok {
		// La posizione è stata chiusa nel frattempo da un'altra goroutine
		b.mu.Unlock()
		return
	}

	var shouldClose bool
	var closeReason string
	var closePrice float64
	var shouldPartial bool

	switch {
	case last.Low <= pos.Stop:
		shouldClose = true
		closeReason = "STOP_LOSS"
		closePrice = pos.Stop
	case !pos.Partial && last.High >= pos.TP1:
		shouldPartial = true
	case pos.TP2 > 0 && last.High >= pos.TP2:
		shouldClose = true
		closeReason = "TAKE_PROFIT"
		closePrice = pos.TP2
	case sig.Type == strategy.SlowExit:
		shouldClose = true
		closeReason = "SLOW_EXIT"
		closePrice = last.Close
	}

	if shouldPartial {
		b.mu.Unlock()
		b.partialTP(ctx, symbol, pos)
		b.log.Printf("managePosition %s: azione=partial", symbol)
	} else if shouldClose {
		b.mu.Unlock()
		b.closePosition(ctx, symbol, closePrice, closeReason)
		b.log.Printf("managePosition %s: azione=%s", symbol, closeReason)
	} else {
		b.mu.Unlock()
	}
}

func (b *Bot) partialTP(ctx context.Context, symbol string, pos *Position) {
	b.mu.Lock()
	defer b.mu.Unlock()

	size := pos.Size * b.params[symbol].PartialPct
	if size <= 0 {
		return
	}
	px, _ := b.conn.LastPrice(ctx, symbol)
	if px <= 0 {
		px = pos.TP1
	}
	res, err := b.conn.MarketOrder(ctx, connector.OrderRequest{
		Symbol: symbol, Side: connector.Sell, Size: size, Price: px,
	})
	if err != nil || res.Status != connector.StatusFilled {
		b.log.Printf("parziale %s rifiutato: err=%v res=%+v", symbol, err, res)
		return
	}
	fill := res.FilledPrice
	if fill <= 0 {
		fill = px
	}
	fee := fill * res.FilledSize * b.cfg.Bot.Commission
	pnl := (fill-pos.EntryPrice)*res.FilledSize - fee
	pos.Size -= res.FilledSize
	pos.Partial = true
	if b.params[symbol].BreakevenOnTP1 {
		pos.Stop = pos.EntryPrice
	}
	b.state.DailyPnL += pnl
	b.state.RealizedPnL += pnl
	b.saveStateLocked()
	b.log.Printf("PARZIALE %s @ %.2f pnl %.2f (stop -> breakeven)", symbol, fill, pnl)
	_ = b.tg.Send(ctx, fmt.Sprintf("💰 <b>PARTIAL TP</b> %s @ %.2f\n📈 PnL parziale: %.2f",
		symbol, fill, pnl))
}

func (b *Bot) closePosition(ctx context.Context, symbol string, refPrice float64, reason string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	pos, ok := b.state.Positions[symbol]
	if !ok {
		return
	}
	px, _ := b.conn.LastPrice(ctx, symbol)
	if px <= 0 {
		px = refPrice
	}
	res, err := b.conn.MarketOrder(ctx, connector.OrderRequest{
		Symbol: symbol, Side: connector.Sell, Size: pos.Size, Price: px,
	})
	if err != nil || res.Status != connector.StatusFilled {
		b.log.Printf("chiusura %s rifiutata: err=%v res=%+v", symbol, err, res)
		return
	}
	fill := res.FilledPrice
	if fill <= 0 {
		fill = px
	}
	fee := fill*res.FilledSize*b.cfg.Bot.Commission + pos.OpenFee
	pnl := (fill-pos.EntryPrice)*res.FilledSize - fee
	now := time.Now()
	b.risk.RecordPnL(now, b.equityUnlocked(ctx), pnl)
	b.state.DailyPnL += pnl
	b.state.DailyTrades++
	b.state.ClosedTrades++
	b.state.RealizedPnL += pnl
	// streak di perdite -> pausa
	if pnl < 0 {
		b.state.ConsecLosses++
	} else {
		b.state.ConsecLosses = 0
	}
	if n := b.risks[symbol].LossStreakN; n > 0 && b.state.ConsecLosses >= n {
		tfMin, _ := market.Minutes(b.cfg.Trading.Timeframe)
		if tfMin <= 0 {
			tfMin = 240
		}
		pauseBars := b.risks[symbol].LossStreakPauseBars
		if pauseBars <= 0 {
			pauseBars = 24
		}
		b.state.PauseUntilMs = now.UnixMilli() + int64(pauseBars*tfMin)*60_000
		b.state.ConsecLosses = 0
	}
	delete(b.state.Positions, symbol)
	b.saveStateLocked()
	b.log.Printf("EXIT %s @ %.2f (%s) pnl %.2f", symbol, fill, reason, pnl)
	_ = b.tg.Send(ctx, telegram.FormatExit(symbol, pnl, reason))
}

// savePosition persiste una posizione modificata (es. trailing).
func (b *Bot) savePosition(pos *Position) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.state.Positions[pos.Symbol]; ok {
		b.state.Positions[pos.Symbol] = pos
		b.saveStateLocked()
	}
}

// Symbols ordinati dei simboli configurati.
func (b *Bot) Symbols() []string {
	out := append([]string{}, b.cfg.Trading.Symbols...)
	sort.Strings(out)
	return out
}
