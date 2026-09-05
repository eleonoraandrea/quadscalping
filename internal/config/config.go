// Package config carica la configurazione del bot (JSON + override da env).
package config

import (
	"encoding/json"
	"os"

	"quadscalping/internal/backtest"
	"quadscalping/internal/strategy"
)

// Exchange configura il connettore ordini.
type Exchange struct {
	ID                string `json:"id"`
	Testnet           bool   `json:"testnet"`
	APIKey            string `json:"api_key"`
	APISecret         string `json:"api_secret"`
	QuantityPrecision int    `json:"quantity_precision"`
	DryRun            bool   `json:"dry_run"`
}

// Trading parametri generali.
type Trading struct {
	Symbols         []string `json:"symbols"`
	Timeframe       string   `json:"timeframe"`
	MaxPositions    int      `json:"max_positions"`
	RiskPct         float64  `json:"risk_pct"`
	MaxDailyLossPct float64  `json:"max_daily_loss_pct"`
}

// HPS parametri strategia (nil = default).
type HPS struct {
	StopMode       *string  `json:"stop_mode,omitempty"`
	StopATR        *float64 `json:"stop_atr,omitempty"`
	StopLookback   *int     `json:"stop_lookback,omitempty"`
	TP1R           *float64 `json:"tp1_r,omitempty"`
	TP2R           *float64 `json:"tp2_r,omitempty"`
	PartialPct     *float64 `json:"partial_pct,omitempty"`
	BreakevenOnTP1 *bool    `json:"breakeven_on_tp1,omitempty"`
	ExitSlow       *float64 `json:"exit_slow,omitempty"`
	MinStrength    *float64 `json:"min_signal_strength,omitempty"`
	VWAPPeriod     *int     `json:"vwap_period,omitempty"`
	RegimeFilter   *string  `json:"regime_filter,omitempty"`
}

// Telegram configurazione notifiche.
type Telegram struct {
	Enabled  bool   `json:"enabled"`
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

// Bot parametri runtime.
type Bot struct {
	PollIntervalSec int     `json:"poll_interval_sec"`
	StateFile       string  `json:"state_file"`
	DataDir         string  `json:"data_dir"`
	HTTPAddr        string  `json:"http_addr"`
	InitialCapital  float64 `json:"initial_capital"` // per paper mode
	Commission      float64 `json:"commission"`      // paper mode
	Slippage        float64 `json:"slippage"`        // paper mode
}

// Risk configura il money management (tutto opzionale, default off).
type Risk struct {
	VolAdjust           bool    `json:"vol_adjust"`             // riduci size se ATR > media
	VolLookback         int     `json:"vol_lookback"`           // default 100
	StrengthSizing      bool    `json:"strength_sizing"`        // scala size con la forza segnale
	DDThrottlePct       float64 `json:"dd_throttle_pct"`        // rischio scende col drawdown
	TrailATRMult        float64 `json:"trail_atr_mult"`         // trailing dopo TP1 (0 = breakeven)
	LossStreakN         int     `json:"loss_streak_n"`          // pausa dopo N perdite consecutive
	LossStreakPauseBars int     `json:"loss_streak_pause_bars"` // default 24
}

// Config è la radice.
type Config struct {
	Exchange Exchange `json:"exchange"`
	Trading  Trading  `json:"trading"`
	HPS      HPS      `json:"hps"`
	// SymbolOverrides ridefinisce i parametri HPS per singolo simbolo
	// (chiave = simbolo, es. "BTCUSDT"). Ha la precedenza sulla sezione hps.
	SymbolOverrides map[string]HPS `json:"symbol_overrides"`
	Risk            Risk           `json:"risk"`
	// RiskOverrides ridefinisce il money management per simbolo.
	RiskOverrides map[string]Risk `json:"risk_overrides"`
	Telegram      Telegram        `json:"telegram"`
	Bot           Bot             `json:"bot"`
}

// Default ritorna la configurazione di default (testnet + dry run).
func Default() Config {
	return Config{
		Exchange: Exchange{ID: "binance", Testnet: true, QuantityPrecision: 6, DryRun: true},
		Trading: Trading{
			Symbols: []string{"BTCUSDT"}, Timeframe: "5m",
			MaxPositions: 3, RiskPct: 0.01, MaxDailyLossPct: 0.05,
		},
		HPS:      HPS{},
		Telegram: Telegram{Enabled: false},
		Bot: Bot{
			PollIntervalSec: 30, StateFile: "data/state.json",
			DataDir: "data", HTTPAddr: ":8080",
			InitialCapital: 10000, Commission: 0.0004, Slippage: 0.0001,
		},
	}
}

// Load carica il file JSON sopra i default. Variabili d'ambiente
// BINANCE_API_KEY / BINANCE_API_SECRET / TELEGRAM_BOT_TOKEN / TELEGRAM_CHAT_ID
// fanno override dei campi corrispondenti.
func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return cfg, err
		}
		if err := json.Unmarshal(b, &cfg); err != nil {
			return cfg, err
		}
	}
	if v := os.Getenv("BINANCE_API_KEY"); v != "" {
		cfg.Exchange.APIKey = v
	}
	if v := os.Getenv("BINANCE_API_SECRET"); v != "" {
		cfg.Exchange.APISecret = v
	}
	if v := os.Getenv("TELEGRAM_BOT_TOKEN"); v != "" {
		cfg.Telegram.BotToken = v
	}
	if v := os.Getenv("TELEGRAM_CHAT_ID"); v != "" {
		cfg.Telegram.ChatID = v
	}
	return cfg, nil
}

// StrategyParams converte la sezione hps in strategy.Params.
func (c Config) StrategyParams() strategy.Params {
	return c.hpsParams(c.HPS)
}

// StrategyParamsFor ritorna i parametri HPS per il simbolo
// (symbol_overrides vince sulla sezione hps globale).
func (c Config) StrategyParamsFor(symbol string) strategy.Params {
	if h, ok := c.SymbolOverrides[symbol]; ok {
		return c.hpsParams(h)
	}
	return c.hpsParams(c.HPS)
}

func (c Config) hpsParams(h HPS) strategy.Params {
	p := strategy.DefaultParams()
	if v := h.StopMode; v != nil {
		p.StopMode = *v
	}
	if v := h.StopATR; v != nil {
		p.StopATR = *v
	}
	if v := h.StopLookback; v != nil {
		p.StopLookback = *v
	}
	if v := h.TP1R; v != nil {
		p.TP1R = *v
	}
	if v := h.TP2R; v != nil {
		p.TP2R = *v
	}
	if v := h.PartialPct; v != nil {
		p.PartialPct = *v
	}
	if v := h.BreakevenOnTP1; v != nil {
		p.BreakevenOnTP1 = *v
	}
	if v := h.ExitSlow; v != nil {
		p.ExitSlow = *v
	}
	if v := h.MinStrength; v != nil {
		p.MinStrength = *v
	}
	if v := h.VWAPPeriod; v != nil {
		p.VWAPPeriod = *v
	}
	if v := h.RegimeFilter; v != nil {
		p.RegimeFilter = *v
	}
	return p
}

// BacktestConfig costruisce la config del backtest dal file config.
func (c Config) BacktestConfig() backtest.Config {
	bc := backtest.DefaultConfig()
	bc.RiskPct = c.Trading.RiskPct
	bc.Params = c.StrategyParams()
	return bc
}

// RiskConfigFor ritorna il money management per il simbolo
// (risk_overrides vince sulla sezione risk globale).
func (c Config) RiskConfigFor(symbol string) Risk {
	if r, ok := c.RiskOverrides[symbol]; ok {
		return r
	}
	return c.Risk
}

// BacktestConfigFor come BacktestConfig ma con gli override del simbolo
// e la sezione risk (money management).
func (c Config) BacktestConfigFor(symbol string) backtest.Config {
	bc := backtest.DefaultConfig()
	bc.RiskPct = c.Trading.RiskPct
	bc.Params = c.StrategyParamsFor(symbol)
	r := c.RiskConfigFor(symbol)
	bc.VolAdjust = r.VolAdjust
	bc.VolLookback = r.VolLookback
	if bc.VolLookback <= 0 {
		bc.VolLookback = 100
	}
	bc.StrengthSizing = r.StrengthSizing
	bc.DDThrottlePct = r.DDThrottlePct
	bc.TrailATRMult = r.TrailATRMult
	bc.LossStreakN = r.LossStreakN
	bc.LossStreakPauseBars = r.LossStreakPauseBars
	if bc.LossStreakPauseBars <= 0 {
		bc.LossStreakPauseBars = 24
	}
	return bc
}
