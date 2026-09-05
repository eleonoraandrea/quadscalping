package ai

import (
	"math"
	"quadscalping/internal/market"
)

// MarkovState rappresenta gli stati di mercato nel modello di Markov
type MarkovState int

const (
	StateStrongBull MarkovState = iota // Toro forte
	StateWeakBull                       // Toro debole
	StateWeakBear                       // Orso debole
	StateStrongBear                     // Orso forte
	StateTransition                     // Transizione incerta
)

// TransitionMatrix è la matrice di transizione 5x5
type TransitionMatrix [5][5]float64

// MarkovConfig configura il filtro di Markov
type MarkovConfig struct {
	LookbackPeriod    int     // Periodo per calcolo probabilità
	ConfidenceThreshold float64 // Soglia minima confidence (0.0-1.0)
	MinTradeProbability float64 // Probabilità minima per entrare
}

// DefaultMarkovConfig restituisce configurazione default ottimizzata
func DefaultMarkovConfig() MarkovConfig {
	return MarkovConfig{
		LookbackPeriod:      100,
		ConfidenceThreshold: 0.65,
		MinTradeProbability: 0.55,
	}
}

// MarkovEngine è il motore AI basato su Catene di Markov
type MarkovEngine struct {
	config           MarkovConfig
	currentState     MarkovState
	transitionMatrix TransitionMatrix
	stateHistory     []MarkovState
	confidence       float64
}

// NewMarkovEngine crea un nuovo engine di Markov
func NewMarkovEngine(cfg MarkovConfig) *MarkovEngine {
	engine := &MarkovEngine{
		config:       cfg,
		currentState: StateTransition,
		stateHistory: make([]MarkovState, 0),
	}
	// Inizializza matrice con probabilità uniformi
	for i := range engine.transitionMatrix {
		for j := range engine.transitionMatrix[i] {
			engine.transitionMatrix[i][j] = 0.2
		}
	}
	return engine
}

// detectState rileva lo stato corrente basato su candele
func (me *MarkovEngine) detectState(candles []market.Candle) MarkovState {
	if len(candles) < me.config.LookbackPeriod {
		return StateTransition
	}

	// Calcola trend e momentum sulle ultime candele
	recentCandles := candles[len(candles)-me.config.LookbackPeriod:]
	
	// Calcolo rendimenti
	returns := make([]float64, len(recentCandles)-1)
	for i := 1; i < len(recentCandles); i++ {
		returns[i-1] = (recentCandles[i].Close - recentCandles[i-1].Close) / recentCandles[i-1].Close
	}

	// Media e deviazione standard dei rendimenti
	meanReturn := 0.0
	for _, r := range returns {
		meanReturn += r
	}
	meanReturn /= float64(len(returns))

	variance := 0.0
	for _, r := range returns {
		diff := r - meanReturn
		variance += diff * diff
	}
	stdDev := math.Sqrt(variance / float64(len(returns)))

	// Z-score del rendimento medio
	zScore := 0.0
	if stdDev > 0 {
		zScore = meanReturn / stdDev
	}

	// Determina stato basato su z-score
	if zScore > 1.5 {
		return StateStrongBull
	} else if zScore > 0.5 {
		return StateWeakBull
	} else if zScore < -1.5 {
		return StateStrongBear
	} else if zScore < -0.5 {
		return StateWeakBear
	}
	return StateTransition
}

// updateTransitionMatrix aggiorna la matrice di transizione
func (me *MarkovEngine) updateTransitionMatrix(from, to MarkovState) {
	me.transitionMatrix[from][to] += 1.0
	
	// Normalizza le righe
	for i := range me.transitionMatrix {
		sum := 0.0
		for j := range me.transitionMatrix[i] {
			sum += me.transitionMatrix[i][j]
		}
		if sum > 0 {
			for j := range me.transitionMatrix[i] {
				me.transitionMatrix[i][j] /= sum
			}
		}
	}
}

// Analyze analizza il mercato e restituisce segnale AI
func (me *MarkovEngine) Analyze(candles []market.Candle) (state MarkovState, confidence float64, canTrade bool) {
	newState := me.detectState(candles)
	
	// Aggiorna storico
	if len(me.stateHistory) > 0 {
		prevState := me.stateHistory[len(me.stateHistory)-1]
		if prevState != StateTransition || newState != StateTransition {
			me.updateTransitionMatrix(prevState, newState)
		}
	}
	
	me.stateHistory = append(me.stateHistory, newState)
	if len(me.stateHistory) > me.config.LookbackPeriod {
		me.stateHistory = me.stateHistory[1:]
	}
	
	me.currentState = newState
	
	// Calcola probabilità di transizione verso stati favorevoli
	bullProb := me.transitionMatrix[newState][StateStrongBull] + 
	            me.transitionMatrix[newState][StateWeakBull] * 0.7
	
	bearProb := me.transitionMatrix[newState][StateStrongBear] + 
	            me.transitionMatrix[newState][StateWeakBear] * 0.7
	
	// Confidence basata sulla frequenza dello stato corrente
	stateCount := 0
	for _, s := range me.stateHistory {
		if s == newState {
			stateCount++
		}
	}
	me.confidence = float64(stateCount) / float64(len(me.stateHistory))
	
	// Decisione di trading
	canTrade = me.confidence >= me.config.ConfidenceThreshold
	
	// Bias direzionale
	if bullProb > bearProb && bullProb > me.config.MinTradeProbability {
		canTrade = true
	} else if bearProb > bullProb && bearProb > me.config.MinTradeProbability {
		canTrade = false // In short mode (se supportato)
	} else {
		canTrade = false
	}
	
	return newState, me.confidence, canTrade
}

// GetState restituisce lo stato corrente
func (me *MarkovEngine) GetState() MarkovState {
	return me.currentState
}

// GetConfidence restituisce la confidence corrente
func (me *MarkovEngine) GetConfidence() float64 {
	return me.confidence
}

// GetTransitionMatrix restituisce la matrice di transizione
func (me *MarkovEngine) GetTransitionMatrix() TransitionMatrix {
	return me.transitionMatrix
}
