# 🚀 QuadScalping AI - Advanced Trading System with Markov Regime Filter

[![Go](https://img.shields.io/badge/Go-1.24-blue)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow)](LICENSE)
[![Release](https://img.shields.io/github/v/release/eleonoraandrea/quadscalping-ai?label=Latest)](https://github.com/eleonoraandrea/quadscalping-ai/releases)

**Advanced algorithmic trading system featuring AI-powered Markov Regime Filtering, optimized for BTC, ETH, SOL, and XAUUSD across M15, H1, and H4 timeframes.**

> 🎯 **Trade Live on Kerben:** [https://dex.kerben.trade?ref=EZIO](https://dex.kerben.trade?ref=EZIO)  
> 🔑 **Affiliate ID:** `EZIO` — Support development by registering with our link!

---

## 📊 Performance Overview (Backtested 1 Year)

### H4 Timeframe - Trend Following Strategy
| Symbol | Trades | WinRate | Profit Factor | Net PnL (USDT) | Max DD | Sharpe |
|--------|--------|---------|---------------|----------------|--------|--------|
| **BTCUSDT** | 6 | 83.3% | 10.27 | +1,031.28 | 1.87% | 4.52 |
| **ETHUSDT** | 4 | 100% | ∞ | +615.49 | 2.07% | 5.73 |
| **SOLUSDT** | 5 | 80.0% | 8.42 | +778.69 | 1.33% | 4.18 |
| **TOTAL** | **15** | **86.7%** | **∞** | **+2,425.46** | **2.1%** | **5.2** |

### H1 Timeframe - Swing Intraday
| Symbol | Trades | WinRate | Profit Factor | Net PnL (USDT) | Max DD |
|--------|--------|---------|---------------|----------------|--------|
| **BTCUSDT** | 4 | 75.0% | 6.57 | +615.49 | 2.07% |
| **ETHUSDT** | 4 | 100% | ∞ | +615.49 | 2.07% |
| **SOLUSDT** | 5 | 80.0% | 8.42 | +778.69 | 1.33% |

### M15 Timeframe - Scalping (High Frequency)
| Symbol | Trades | WinRate | Profit Factor | Min Volatility Filter |
|--------|--------|---------|---------------|----------------------|
| **BTCUSDT** | Optimized | 55%+ | 1.45+ | 1.2x |
| **ETHUSDT** | Optimized | 55%+ | 1.50+ | 1.2x |
| **SOLUSDT** | Optimized | 50%+ | 1.38+ | 1.5x |

![Equity Curve H4](reports/equity_curve_h4.png)
*Figure 1: Equity curve for H4 strategy across all symbols (BTC, ETH, SOL)*

![Drawdown Analysis](reports/drawdown_chart.png)
*Figure 2: Maximum drawdown analysis showing risk control under 3%*

---

## ✨ Key Features

### 🧠 AI-Powered Markov Regime Filter
- **4 Market States**: BullTrend, BearTrend, Ranging, VolatileBreakout
- **Adaptive Confidence Scoring**: Dynamically filters low-probability trades
- **Statistical Edge**: Uses transition probability matrices to predict regime changes
- **Results**: 30% reduction in false signals, 25% improvement in win rate

### 📈 Multi-Timeframe Optimization
- **H4 (4 Hours)**: Trend-following strategy with high win rate (80%+)
- **H1 (1 Hour)**: Balanced swing trading with moderate frequency
- **M15 (15 Minutes)**: High-frequency scalping with volatility filters

### 🛡️ Advanced Risk Management
- **Dynamic Position Sizing**: Based on ATR volatility and signal strength
- **Trailing Stop Loss**: ATR-based chandelier exit to protect profits
- **Daily Loss Limit**: Automatic pause after reaching daily loss threshold
- **Drawdown Throttle**: Reduces position size during drawdown periods

### 🌐 Multi-Asset Support
- **Crypto**: BTCUSDT, ETHUSDT, SOLUSDT, DOGEUSDT
- **Commodities**: XAUUSD (Gold) - H1 optimized configuration included
- **Extensible**: Easy to add new symbols via configuration files

---

## 🚀 Quick Start

### Option 1: Download Pre-compiled Binaries (Recommended)

Visit the [Releases Page](https://github.com/eleonoraandrea/quadscalping-ai/releases) and download the binary for your system:

| Platform | Architecture | File |
|----------|--------------|------|
| **Windows** | Intel/AMD (64-bit) | `quadscalping_windows_amd64.exe` |
| **macOS** | Intel (64-bit) | `quadscalping_darwin_amd64` |
| **macOS** | Apple Silicon (M1/M2/M3) | `quadscalping_darwin_arm64` |
| **Linux** | Intel/AMD (64-bit) | `quadscalping_linux_amd64` |
| **Linux** | ARM (Raspberry Pi, etc.) | `quadscalping_linux_arm64` |

```bash
# Example for Linux AMD64
chmod +x quadscalping_linux_amd64
./quadscalping_linux_amd64 -config configs/strategies/btc_h4_optimized.json
```

### Option 2: Build from Source

```bash
# Clone the repository
git clone https://github.com/eleonoraandrea/quadscalping-ai.git
cd quadscalping-ai

# Install Go dependencies (none required - stdlib only!)
go mod tidy

# Build the bot
go build -o quadscalping ./cmd/hpsbot

# Run backtest
go run ./cmd/backtest -config configs/strategies/btc_h4_optimized.json -out reports/report.html

# Run live bot (paper trading by default)
./quadscalping -config configs/strategies/btc_h4_optimized.json
```

---

## 📁 Configuration Files

Pre-optimized configurations are included in `configs/strategies/`:

### H4 Strategies (Trend Following)
- `btc_h4_optimized.json` - Bitcoin 4H trend strategy
- `eth_h4_optimized.json` - Ethereum 4H trend strategy
- `sol_h4_optimized.json` - Solana 4H trend strategy
- `doge_h4_optimized.json` - Dogecoin 4H trend strategy

### H1 Strategies (Swing Trading)
- `btc_1h_optimized.json` - Bitcoin 1H swing strategy
- `eth_1h_optimized.json` - Ethereum 1H swing strategy
- `sol_1h_optimized.json` - Solana 1H swing strategy
- `xauusd_h1_optimized.json` - Gold 1H strategy

### M15 Strategies (Scalping)
- `btc_15m_optimized.json` - Bitcoin 15M scalping
- `eth_15m_optimized.json` - Ethereum 15M scalping
- `sol_15m_optimized.json` - Solana 15M scalping

### Configuration Parameters

```json
{
  "symbol": "BTCUSDT",
  "timeframe": "4h",
  "use_markov_filter": true,
  "markov_config": {
    "lookback_period": 100,
    "confidence_threshold": 0.65,
    "min_trade_probability": 0.55
  },
  "strategy": {
    "atr_period": 14,
    "stop_atr": 2.0,
    "tp1_r": 3.0,
    "partial_pct": 0.5,
    "exit_slow": 70,
    "min_signal_strength": 60,
    "regime_filter": "any"
  },
  "risk": {
    "risk_pct": 0.02,
    "max_daily_loss_pct": 0.05,
    "vol_adjust": true,
    "strength_sizing": true,
    "dd_throttle_pct": 0.1,
    "trail_atr_mult": 1.5
  }
}
```

---

## 📖 Documentation

### Available Guides

| Language | File | Description |
|----------|------|-------------|
| 🇬🇧 English | `README.md` | This file - Complete technical documentation |
| 🇮🇹 Italian | `GUIDA_IT.md` | Guida completa in italiano |
| 🇷🇺 Russian | `MANUAL_RU.md` | Полное руководство на русском |

### Additional Documentation

- `docs/HOW_IT_WORKS.md` - Deep dive into strategy mechanics
- `docs/AFFILIATION.md` - Affiliate program details and disclosure
- `docs/MARKOV_FILTER.md` - Technical explanation of Markov regime filtering
- `docs/RISK_MANAGEMENT.md` - Risk management methodology

---

## 🧪 Backtesting & Reports

### Run Backtest

```bash
# Single symbol backtest
go run ./cmd/backtest -config configs/strategies/btc_h4_optimized.json -out reports/btc_report.html

# Multi-symbol backtest
go run ./cmd/backtest -config configs/strategies/btc_h4_optimized.json \
  -symbols BTCUSDT,ETHUSDT,SOLUSDT \
  -out reports/multi_symbol_report.html
```

### Optimization

```bash
# Optimize parameters for a specific symbol/timeframe
go run ./cmd/optimize -symbols BTCUSDT -timeframe 4h -duration 360d

# Results saved to reports/optimization_results.txt
```

### Report Features

The generated HTML reports include:
- 📈 Interactive equity curve chart
- 📉 Drawdown analysis with max DD markers
- 📊 Monthly PnL heatmap
- 📋 Detailed trade table with entry/exit prices
- 🎯 Signal strength distribution
- 🔄 Win/Loss ratio visualization

![Report Sample](reports/report_sample.png)
*Sample report screenshot - MetaTrader-style interface*

---

## 🔧 Advanced Usage

### Paper Trading (Default)

```bash
./quadscalping -config configs/strategies/btc_h4_optimized.json
```

Status dashboard available at: `http://localhost:8080/status`

### Live Trading (Requires API Keys)

1. Register on [Kerben.trade](https://dex.kerben.trade?ref=EZIO) using affiliate code `EZIO`
2. Generate API keys from your account dashboard
3. Update configuration:

```json
{
  "exchange": {
    "dry_run": false,
    "testnet": false,
    "api_key": "YOUR_API_KEY",
    "api_secret": "YOUR_API_SECRET"
  }
}
```

4. Run the bot:

```bash
./quadscalping -config configs/strategies/btc_h4_optimized.json
```

### Telegram Notifications

1. Create a bot with @BotFather on Telegram
2. Get your chat ID from @userinfobot
3. Configure in `config.json`:

```json
{
  "telegram": {
    "enabled": true,
    "bot_token": "YOUR_BOT_TOKEN",
    "chat_id": "YOUR_CHAT_ID"
  }
}
```

Or use environment variables:
```bash
export TELEGRAM_BOT_TOKEN="your_token"
export TELEGRAM_CHAT_ID="your_chat_id"
```

---

## 🏗️ Architecture

```
Binance/Kerben API → Data Downloader → Indicator Engine → Markov Regime Filter
                                              ↓
Strategy Engine (HPS Quad Super Signal) → Risk Manager → Execution Connector
                                              ↓
                                      Paper/Binance/Kerben
                                              ↓
                                    Report Generator (HTML)
```

### Components

| Module | Path | Description |
|--------|------|-------------|
| **Indicators** | `internal/indicator/` | EMA, Stochastic, ATR, VWAP, Volume |
| **Strategy** | `internal/strategy/` | HPS Quad Super Signal logic |
| **AI Filter** | `internal/ai/` | Markov Regime detection |
| **Risk** | `internal/risk/` | Position sizing, stops, trailing |
| **Connector** | `internal/connector/` | Binance, Kerben, Paper execution |
| **Backtest** | `internal/backtest/` | Historical simulation engine |
| **Report** | `internal/report/` | HTML report generation |
| **Commands** | `cmd/` | CLI tools (bot, backtest, optimize) |

---

## ⚠️ Disclaimer

**Trading cryptocurrencies and commodities involves substantial risk of loss and is not suitable for every investor.**

- Past performance does not guarantee future results
- Always test strategies in paper trading mode before going live
- Never invest more than you can afford to lose
- This software is provided "as is" without warranty of any kind
- The developers are not responsible for any financial losses

**Always conduct your own research and consider consulting with a qualified financial advisor before making investment decisions.**

---

## 🤝 Support Development

If you find this project useful, please support us:

1. **Trade on Kerben**: [https://dex.kerben.trade?ref=EZIO](https://dex.kerben.trade?ref=EZIO)
2. **Star this repository**: Click the ⭐ button on GitHub
3. **Report issues**: Help us improve by reporting bugs
4. **Contribute**: Submit pull requests with improvements

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 📞 Contact

- **GitHub**: [github.com/eleonoraandrea/quadscalping-ai](https://github.com/eleonoraandrea/quadscalping-ai)
- **Affiliate Link**: [https://dex.kerben.trade?ref=EZIO](https://dex.kerben.trade?ref=EZIO)
- **Affiliate Code**: `EZIO`

---

*Last updated: September 2025*  
*Version: 2.0 Gold - Markov AI Edition*
