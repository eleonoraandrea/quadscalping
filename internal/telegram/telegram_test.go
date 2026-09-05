package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quadscalping/internal/strategy"
)

func TestNewDisabledWithoutCredentials(t *testing.T) {
	if New("", "", true).Enabled() {
		t.Error("senza token deve essere disabilitato")
	}
	if !New("tok", "chat", true).Enabled() {
		t.Error("con token+chat deve essere abilitato")
	}
	if New("tok", "chat", false).Enabled() {
		t.Error("flag enabled=false deve disabilitare")
	}
}

func TestSendPostsToBotAPI(t *testing.T) {
	var gotPath string
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &gotBody)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	n := New("123:ABC", "42", true)
	n.baseURL = srv.URL
	if err := n.Send(context.Background(), "ciao <b>mondo</b>"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/bot123:ABC/sendMessage" {
		t.Errorf("path %v", gotPath)
	}
	if gotBody["chat_id"] != "42" || gotBody["text"] != "ciao <b>mondo</b>" ||
		gotBody["parse_mode"] != "HTML" {
		t.Errorf("body %v", gotBody)
	}
}

func TestSendNoopWhenDisabled(t *testing.T) {
	n := New("", "", true)
	if err := n.Send(context.Background(), "x"); err != nil {
		t.Errorf("disabilitato non deve dare errore: %v", err)
	}
}

func TestFormatSignal(t *testing.T) {
	sig := strategy.Signal{
		Type: strategy.BuyEntry, Strength: 82, Regime: strategy.Down,
		EntryPrice: 42000.5, StopPrice: 41850.25, TP1: 42225.9,
	}
	msg := FormatSignal("BTCUSDT", sig)
	for _, want := range []string{"BTCUSDT", "82", "42000", "41850", "42225", "BUY_ENTRY"} {
		if !strings.Contains(msg, want) {
			t.Errorf("messaggio senza %q: %s", want, msg)
		}
	}
}

func TestFormatExit(t *testing.T) {
	msg := FormatExit("BTCUSDT", -123.45, "STOP_LOSS")
	for _, want := range []string{"BTCUSDT", "-123.45", "STOP_LOSS"} {
		if !strings.Contains(msg, want) {
			t.Errorf("messaggio senza %q: %s", want, msg)
		}
	}
}
