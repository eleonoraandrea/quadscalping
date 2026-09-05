package data

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"quadscalping/internal/market"
)

const sampleKlines = `[
  [1700000000000, "42000.10", "42100.50", "41950.00", "42050.25", "123.456", 1700000299999, "1", 10, "0.1", "0"],
  [1700000300000, "42050.25", "42200.00", "42010.00", "42150.00", "200.000", 1700000599999, "1", 10, "0.1", "0"]
]`

func TestParseKlines(t *testing.T) {
	cs, err := ParseKlines([]byte(sampleKlines))
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 {
		t.Fatalf("len %d", len(cs))
	}
	if cs[0].Time != 1700000000000 || cs[0].Open != 42000.10 || cs[0].High != 42100.50 ||
		cs[0].Low != 41950.00 || cs[0].Close != 42050.25 || cs[0].Volume != 123.456 {
		t.Errorf("candela 0 sbagliata: %+v", cs[0])
	}
	if cs[1].Time != 1700000300000 {
		t.Errorf("time 1: %v", cs[1].Time)
	}
}

func TestParseKlinesRejectsBadPayload(t *testing.T) {
	if _, err := ParseKlines([]byte(`{"code":-1121}`)); err == nil {
		t.Error("payload non-array deve fallire")
	}
}

func TestFetchKlinesPaginates(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if r.URL.Path != "/api/v3/klines" {
			t.Errorf("path %v", r.URL.Path)
		}
		if q.Get("symbol") != "BTCUSDT" || q.Get("interval") != "5m" || q.Get("limit") != "1000" {
			t.Errorf("params sbagliati: %v", q)
		}
		st, _ := strconv.ParseInt(q.Get("startTime"), 10, 64)
		var page string
		switch st {
		case 0:
			page = klinePage(0, 1000) // pagina piena -> continua
		case 999*300000 + 1:
			page = klinePage(999*300000+1, 2) // pagina corta -> fine
		default:
			t.Errorf("startTime inatteso: %d", st)
			page = `[]`
		}
		fmt.Fprint(w, page)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), Pause: time.Millisecond}
	cs, err := c.FetchKlines(context.Background(), "BTCUSDT", "5m", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("calls %d want 2", calls)
	}
	if len(cs) != 1002 {
		t.Fatalf("candles %d want 1002", len(cs))
	}
	for i := 1; i < len(cs); i++ {
		if cs[i].Time <= cs[i-1].Time {
			t.Errorf("non ordinate a %d", i)
		}
	}
}

// klinePage genera n candele a partire da t0 con passo 300000 ms.
func klinePage(t0 int64, n int) string {
	rows := make([][]interface{}, n)
	for i := 0; i < n; i++ {
		t := t0 + int64(i)*300000
		rows[i] = []interface{}{t, "1", "2", "0.5", "1.5", "10", t + 299999, "x", 0, "0", "0"}
	}
	b, _ := json.Marshal(rows)
	return string(b)
}

func makeTestCandles(n int) (out []market.Candle) {
	for i := 0; i < n; i++ {
		t := int64(i) * 300000
		out = append(out, market.Candle{
			Time: t, Open: float64(i) + 1.5, High: float64(i) + 2.25,
			Low: float64(i) + 0.75, Close: float64(i) + 1.125, Volume: float64(i) * 3.5,
		})
	}
	return out
}

func TestCSVRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "BTCUSDT_5m.csv")
	want := makeTestCandles(5)
	if err := SaveCSV(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCSV(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("len %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candela %d: %+v != %+v", i, got[i], want[i])
		}
	}
}

func TestLoadCSVMissing(t *testing.T) {
	if _, err := LoadCSV(filepath.Join(t.TempDir(), "nope.csv")); err == nil {
		t.Error("file mancante deve dare errore")
	}
}

func TestMergeDedup(t *testing.T) {
	base := makeTestCandles(3)      // t=0,300000,600000
	dup := makeTestCandles(2)       // t=0,300000
	extra := makeTestCandles(4)[3:] // t=900000
	merged := Merge(base, append(append([]market.Candle{}, dup...), extra...))
	if len(merged) != 4 {
		t.Fatalf("len %d want 4", len(merged))
	}
	if merged[3].Time != 900000 {
		t.Errorf("ultimo time %v", merged[3].Time)
	}
}

func TestUpdateCSVIncremental(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		st, _ := strconv.ParseInt(r.URL.Query().Get("startTime"), 10, 64)
		if st == 0 {
			fmt.Fprint(w, klinePage(0, 2))
		} else {
			// cache fino a t=300000 -> richiede da 300001
			fmt.Fprint(w, klinePage(300001, 2))
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "BTCUSDT_5m.csv")

	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	cs1, err := UpdateCSV(context.Background(), path, c, "BTCUSDT", "5m")
	if err != nil {
		t.Fatal(err)
	}
	if len(cs1) != 2 {
		t.Fatalf("primo update: %d want 2", len(cs1))
	}
	cs2, err := UpdateCSV(context.Background(), path, c, "BTCUSDT", "5m")
	if err != nil {
		t.Fatal(err)
	}
	if len(cs2) != 4 {
		t.Fatalf("secondo update: %d want 4", len(cs2))
	}
	if cs2[0].Time != 0 || cs2[3].Time != 600001 {
		t.Errorf("tempi sbagliati: %v..%v", cs2[0].Time, cs2[3].Time)
	}
	if _, err := os.Stat(path); err != nil {
		t.Error("file cache non creato")
	}
}
