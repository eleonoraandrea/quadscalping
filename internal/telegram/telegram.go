// Package telegram invia notifiche al bot Telegram.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"quadscalping/internal/strategy"
)

// Notifier invia messaggi HTML via Bot API.
type Notifier struct {
	token   string
	chatID  string
	enabled bool
	hc      *http.Client
	baseURL string // override per test
}

// New crea il notifier; disabilitato se token o chat mancanti.
func New(token, chatID string, enabled bool) *Notifier {
	n := &Notifier{
		token:   token,
		chatID:  chatID,
		enabled: enabled && token != "" && chatID != "",
		hc:      &http.Client{Timeout: 10 * time.Second},
		baseURL: "https://api.telegram.org",
	}
	return n
}

// Enabled dice se le notifiche sono attive.
func (n *Notifier) Enabled() bool { return n != nil && n.enabled }

// Send invia un messaggio (parse mode HTML).
func (n *Notifier) Send(ctx context.Context, text string) error {
	if !n.Enabled() {
		return nil
	}
	payload, err := json.Marshal(map[string]string{
		"chat_id":    n.chatID,
		"text":       text,
		"parse_mode": "HTML",
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		n.baseURL+"/bot"+n.token+"/sendMessage", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.hc.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram: status %s", resp.Status)
	}
	return nil
}

// FormatSignal compone il messaggio per un nuovo segnale/entry.
func FormatSignal(symbol string, sig strategy.Signal) string {
	return fmt.Sprintf(
		"🎯 <b>HPS %v</b>\n📊 <b>Asset:</b> %s\n💪 <b>Strength:</b> %.0f/100\n"+
			"📈 <b>Entry:</b> %.2f\n🛑 <b>Stop:</b> %.2f\n🎯 <b>TP1:</b> %.2f\n"+
			"🌍 <b>Regime:</b> %v",
		sig.Type, symbol, sig.Strength, sig.EntryPrice, sig.StopPrice, sig.TP1, sig.Regime)
}

// FormatExit compone il messaggio di chiusura posizione.
func FormatExit(symbol string, pnl float64, reason string) string {
	emoji := "✅"
	if pnl < 0 {
		emoji = "❌"
	}
	return fmt.Sprintf("%s <b>POSIZIONE CHIUSA</b>\n📊 %s\n📈 PnL: %.2f\n📝 %s",
		emoji, symbol, pnl, reason)
}
