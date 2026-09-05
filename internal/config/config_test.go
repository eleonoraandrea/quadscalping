package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Exchange.DryRun || !cfg.Exchange.Testnet {
		t.Error("default deve essere testnet+dryrun")
	}
	if cfg.Trading.RiskPct != 0.01 || cfg.Bot.PollIntervalSec != 30 {
		t.Errorf("default trading/bot sbagliati: %+v", cfg)
	}
	p := cfg.StrategyParams()
	if p.StopATR != 1.5 || p.MinStrength != 50 {
		t.Errorf("strategy default sbagliati: %+v", p)
	}
}

func TestLoadFileAndEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	body := `{
	  "exchange": {"dry_run": false},
	  "trading": {"symbols": ["ETHUSDT","BTCUSDT"], "risk_pct": 0.02},
	  "hps": {"stop_atr": 2.0, "min_signal_strength": 70},
	  "telegram": {"enabled": true, "bot_token": "filetok", "chat_id": "1"},
	  "bot": {"poll_interval_sec": 10}
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BINANCE_API_KEY", "envkey")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Exchange.DryRun {
		t.Error("dry_run false dal file")
	}
	if len(cfg.Trading.Symbols) != 2 || cfg.Trading.Symbols[0] != "ETHUSDT" {
		t.Errorf("symbols %v", cfg.Trading.Symbols)
	}
	if cfg.Trading.RiskPct != 0.02 {
		t.Errorf("risk %v", cfg.Trading.RiskPct)
	}
	if cfg.Exchange.APIKey != "envkey" {
		t.Errorf("env override mancante: %q", cfg.Exchange.APIKey)
	}
	if cfg.Telegram.BotToken != "filetok" {
		t.Errorf("telegram token %q", cfg.Telegram.BotToken)
	}
	p := cfg.StrategyParams()
	if p.StopATR != 2.0 || p.MinStrength != 70 {
		t.Errorf("hps override mancante: %+v", p)
	}
	if cfg.Bot.PollIntervalSec != 10 {
		t.Errorf("poll %d", cfg.Bot.PollIntervalSec)
	}
	// campi non overridati restano default
	if p.TP1R != 1.5 {
		t.Errorf("tp1 default perso: %v", p.TP1R)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load("/non/esiste.json"); err == nil {
		t.Error("file mancante deve dare errore")
	}
}

func TestSymbolOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	body := `{
	  "trading": {"symbols": ["BTCUSDT","ETHUSDT"]},
	  "hps": {"stop_atr": 2.0},
	  "symbol_overrides": {
	    "BTCUSDT": {"stop_atr": 3.0, "regime_filter": "down"},
	    "ETHUSDT": {"stop_atr": 1.5}
	  }
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	btc := cfg.StrategyParamsFor("BTCUSDT")
	if btc.StopATR != 3.0 || btc.RegimeFilter != "down" {
		t.Errorf("override BTC: %+v", btc)
	}
	eth := cfg.StrategyParamsFor("ETHUSDT")
	if eth.StopATR != 1.5 {
		t.Errorf("override ETH: %+v", eth)
	}
	glob := cfg.StrategyParamsFor("SOLUSDT") // nessun override -> globale
	if glob.StopATR != 2.0 {
		t.Errorf("fallback globale: %+v", glob)
	}
	if cfg.StrategyParams().StopATR != 2.0 {
		t.Errorf("globale: %+v", cfg.StrategyParams())
	}
	bc := cfg.BacktestConfigFor("BTCUSDT")
	if bc.Params.StopATR != 3.0 || bc.RiskPct != cfg.Trading.RiskPct {
		t.Errorf("backtest config per simbolo: %+v", bc.Params)
	}
}

func TestRiskOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	body := `{
	  "trading": {"symbols": ["BTCUSDT","ETHUSDT"]},
	  "risk": {"strength_sizing": true},
	  "risk_overrides": {
	    "ETHUSDT": {"vol_adjust": true, "vol_lookback": 50}
	  }
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	btc := cfg.BacktestConfigFor("BTCUSDT")
	if !btc.StrengthSizing || btc.VolAdjust {
		t.Errorf("BTC deve usare risk globale: %+v", btc)
	}
	eth := cfg.BacktestConfigFor("ETHUSDT")
	// l'override è sostituzione completa del risk per simbolo
	if !eth.VolAdjust || eth.StrengthSizing || eth.VolLookback != 50 {
		t.Errorf("ETH deve usare solo l'override: %+v", eth)
	}
	if cfg.RiskConfigFor("ETHUSDT").VolLookback != 50 {
		t.Error("RiskConfigFor override mancante")
	}
}
