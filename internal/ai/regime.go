package ai

import (
	"math"
	"quadscalping/internal/market"
)

// MarketState rappresenta lo stato corrente del mercato rilevato dall'AI
type MarketState int

const (
	StateBullTrend MarketState = iota // Trend Rialzista Forte
	StateBearTrend                    // Trend Ribassista Forte
	StateRanging                      // Mercato Laterale/Choppy
	StateVolatileBreakout             // Alta volatilità, potenziale breakout
)

// RegimeConfig configura la sensibilità dell'engine
type RegimeConfig struct {
	LookbackPeriod       int     // Periodo per il calcolo delle statistiche
	VolatilityThreshold  float64 // Soglia per definire "alta volatilità"
	TrendStrengthThreshold float64 // Soglia per definire un trend forte
}

// DefaultConfig restituisce una configurazione bilanciata
func DefaultConfig() RegimeConfig {
	return RegimeConfig{
		LookbackPeriod:       50,
		VolatilityThreshold:  1.5, // 1.5x la media storica
		TrendStrengthThreshold: 0.6, // Correlazione o pendenza significativa
	}
}

// AdaptiveEngine è il cuore dell'AI che rileva il regime di mercato
type AdaptiveEngine struct {
	config     RegimeConfig
	state      MarketState
	confidence float64 // 0.0 to 1.0
}

// NewAdaptiveEngine crea una nuova istanza
func NewAdaptiveEngine(cfg RegimeConfig) *AdaptiveEngine {
	return &AdaptiveEngine{
		config: cfg,
		state:  StateRanging,
	}
}

// Analyze aggiorna lo stato del mercato basato sui dati recenti
func (ae *AdaptiveEngine) Analyze(candles []market.Candle) MarketState {
	if len(candles) < ae.config.LookbackPeriod {
		return StateRanging
	}

	start := len(candles) - ae.config.LookbackPeriod
	recentClose := make([]float64, ae.config.LookbackPeriod)
	recentHigh := make([]float64, ae.config.LookbackPeriod)
	recentLow := make([]float64, ae.config.LookbackPeriod)
	
	for i := 0; i < ae.config.LookbackPeriod; i++ {
		recentClose[i] = candles[start+i].Close
		recentHigh[i] = candles[start+i].High
		recentLow[i] = candles[start+i].Low
	}

	// 1. Calcolo Statistiche Base
	mean := getMean(recentClose)
	stdDev := getStdDev(recentClose, mean)
	
	// 2. Calcolo Volatilità (ATR semplificato su lookback)
	atr := getAverageTrueRange(recentHigh, recentLow, recentClose)
	
	// 3. Calcolo Pendenza (Trend Strength)
	// Regressione lineare semplice per trovare la pendenza
	slope := getSlope(recentClose)

	// Normalizzazione slope rispetto alla volatilità
	normalizedSlope := slope / (stdDev + 1e-9)

	// 4. Logica di Classificazione (Il "Cervello")
	
	// Controllo Volatilità Estrema - confronto con ATR storico
	avgAtrHistory := atr // default se non abbiamo storico
	if start > 0 && len(candles) > ae.config.LookbackPeriod*2 {
		// Calcola ATR medio sul periodo precedente
		prevStart := start - ae.config.LookbackPeriod
		prevHigh := make([]float64, ae.config.LookbackPeriod)
		prevLow := make([]float64, ae.config.LookbackPeriod)
		prevClose := make([]float64, ae.config.LookbackPeriod)
		for i := 0; i < ae.config.LookbackPeriod; i++ {
			prevHigh[i] = candles[prevStart+i].High
			prevLow[i] = candles[prevStart+i].Low
			prevClose[i] = candles[prevStart+i].Close
		}
		avgAtrHistory = getAverageTrueRange(prevHigh, prevLow, prevClose)
	}
	
	if avgAtrHistory == 0 { avgAtrHistory = atr }
	
	volRatio := atr / avgAtrHistory

	if volRatio > ae.config.VolatilityThreshold {
		ae.state = StateVolatileBreakout
		ae.confidence = math.Min(1.0, volRatio/3.0)
		return ae.state
	}

	// Controllo Trend
	if math.Abs(normalizedSlope) > ae.config.TrendStrengthThreshold {
		if normalizedSlope > 0 {
			ae.state = StateBullTrend
		} else {
			ae.state = StateBearTrend
		}
		ae.confidence = math.Min(1.0, math.Abs(normalizedSlope))
		return ae.state
	}

	// Default: Ranging
	ae.state = StateRanging
	ae.confidence = 1.0 - math.Abs(normalizedSlope)
	return ae.state
}

// GetState restituisce lo stato corrente
func (ae *AdaptiveEngine) GetState() MarketState {
	return ae.state
}

// GetConfidence restituisce la confidenza della previsione
func (ae *AdaptiveEngine) GetConfidence() float64 {
	return ae.confidence
}

// GetAdjustmentFactors restituisce i moltiplicatori per la strategia basati sullo stato
// Restituisce: [TP_Multiplier, SL_Multiplier, Size_Multiplier, MinWinRate_Req]
func (ae *AdaptiveEngine) GetAdjustmentFactors() (float64, float64, float64, float64) {
	switch ae.state {
	case StateBullTrend:
		// Trend forte: Lascia correre i profitti, stop più largo per rumore, size maggiore
		return 1.5, 1.2, 1.2, 0.45 
	case StateBearTrend:
		// Trend forte ribassista: Short aggressivi
		return 1.5, 1.2, 1.2, 0.45
	case StateVolatileBreakout:
		// Breakout: Target molto ampi, stop larghi, size normale (rischio alto)
		return 2.0, 1.5, 1.0, 0.40
	case StateRanging:
		// Laterale: Take profit veloci, stop stretti, size ridotta, richiede winrate alto
		return 0.7, 0.8, 0.5, 0.60
	default:
		return 1.0, 1.0, 1.0, 0.50
	}
}

// Helper Functions
func getMean(data []float64) float64 {
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

func getStdDev(data []float64, mean float64) float64 {
	sum := 0.0
	for _, v := range data {
		diff := v - mean
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(data)))
}

func getSlope(data []float64) float64 {
	n := float64(len(data))
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumXX := 0.0

	for i, y := range data {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}

	denom := n*sumXX - sumX*sumX
	if denom == 0 {
		return 0
	}
	return (n*sumXY - sumX*sumY) / denom
}

func getAverageTrueRange(high, low, close []float64) float64 {
	if len(close) < 2 {
		return 0
	}
	sum := 0.0
	count := 0
	for i := 1; i < len(close); i++ {
		tr := high[i] - low[i]
		hl := math.Abs(high[i] - close[i-1])
		lc := math.Abs(low[i] - close[i-1])
		
		maxVal := tr
		if hl > maxVal { maxVal = hl }
		if lc > maxVal { maxVal = lc }
		
		sum += maxVal
		count++
	}
	if count == 0 { return 0 }
	return sum / float64(count)
}
