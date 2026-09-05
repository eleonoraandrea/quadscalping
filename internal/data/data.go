// Package data scarica e cachea candele OHLCV da Binance (endpoint pubblico spot).
package data

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"quadscalping/internal/market"
)

// ParseKlines converte il payload JSON di /api/v3/klines in candele.
// Ogni riga Binance: [openTime, open, high, low, close, volume, closeTime, ...].
func ParseKlines(payload []byte) ([]market.Candle, error) {
	var rows [][]json.RawMessage
	if err := json.Unmarshal(payload, &rows); err != nil {
		return nil, fmt.Errorf("payload klines non valido: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	cs := make([]market.Candle, 0, len(rows))
	for _, row := range rows {
		if len(row) < 6 {
			return nil, fmt.Errorf("riga kline con %d campi, servono 6", len(row))
		}
		var t float64
		if err := json.Unmarshal(row[0], &t); err != nil {
			return nil, fmt.Errorf("open time non valido: %w", err)
		}
		nums := make([]float64, 5)
		for i := 0; i < 5; i++ {
			s := string(row[i+1])
			s, err := strconv.Unquote(s)
			if err != nil {
				s = string(row[i+1])
			}
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, fmt.Errorf("valore kline %d non valido (%s): %w", i+1, s, err)
			}
			nums[i] = v
		}
		cs = append(cs, market.Candle{
			Time: int64(t), Open: nums[0], High: nums[1], Low: nums[2],
			Close: nums[3], Volume: nums[4],
		})
	}
	return cs, nil
}

// Client scarica klines da Binance.
type Client struct {
	BaseURL string // default https://api.binance.com
	HTTP    *http.Client
	// Pause tra richieste consecutive, per rispettare i rate limit.
	Pause time.Duration
}

// NewClient ritorna un client con default sensati.
func NewClient() *Client {
	return &Client{
		BaseURL: "https://api.binance.com",
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		Pause:   250 * time.Millisecond,
	}
}

// FetchKlines scarica tutte le candele in [startTime, endTime) paginando
// (limit 1000 per richiesta). startTime 0 = più vecchio disponibile;
// endTime 0 = adesso.
func (c *Client) FetchKlines(ctx context.Context, symbol, interval string, startTime, endTime int64) ([]market.Candle, error) {
	base := c.BaseURL
	if base == "" {
		base = "https://api.binance.com"
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	pause := c.Pause

	var out []market.Candle
	cursor := startTime
	for {
		q := url.Values{}
		q.Set("symbol", symbol)
		q.Set("interval", interval)
		q.Set("limit", "1000")
		if cursor > 0 {
			q.Set("startTime", strconv.FormatInt(cursor, 10))
		}
		if endTime > 0 {
			q.Set("endTime", strconv.FormatInt(endTime, 10))
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			base+"/api/v3/klines?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := hc.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("binance %s: %s", resp.Status, truncate(body, 200))
		}
		page, err := ParseKlines(body)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		out = append(out, page...)
		cursor = page[len(page)-1].Time + 1
		if len(page) < 1000 {
			break
		}
		if pause > 0 {
			select {
			case <-time.After(pause):
			case <-ctx.Done():
				return out, ctx.Err()
			}
		}
	}
	return out, nil
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n])
	}
	return string(b)
}

// SaveCSV scrive le candele in formato CSV (header time,open,high,low,close,volume).
func SaveCSV(path string, candles []market.Candle) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"time", "open", "high", "low", "close", "volume"}); err != nil {
		return err
	}
	for _, c := range candles {
		rec := []string{
			strconv.FormatInt(c.Time, 10),
			strconv.FormatFloat(c.Open, 'g', -1, 64),
			strconv.FormatFloat(c.High, 'g', -1, 64),
			strconv.FormatFloat(c.Low, 'g', -1, 64),
			strconv.FormatFloat(c.Close, 'g', -1, 64),
			strconv.FormatFloat(c.Volume, 'g', -1, 64),
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// LoadCSV carica candele da CSV ordinato per time.
func LoadCSV(path string) ([]market.Candle, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = 6
	var out []market.Candle
	first := true
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if first && rec[0] == "time" {
			first = false
			continue
		}
		first = false
		if len(rec) != 6 {
			return nil, fmt.Errorf("%s: riga con %d campi", path, len(rec))
		}
		t, err := strconv.ParseInt(rec[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: time non valido %q: %w", path, rec[0], err)
		}
		vals := make([]float64, 5)
		for i := 0; i < 5; i++ {
			vals[i], err = strconv.ParseFloat(rec[i+1], 64)
			if err != nil {
				return nil, fmt.Errorf("%s: valore %q: %w", path, rec[i+1], err)
			}
		}
		out = append(out, market.Candle{
			Time: t, Open: vals[0], High: vals[1], Low: vals[2],
			Close: vals[3], Volume: vals[4],
		})
	}
	return out, nil
}

// Merge appende newCandles a base deduplicando per Time (entrambe ordinate).
func Merge(base, newCandles []market.Candle) []market.Candle {
	out := base
	if cap(out) < len(out)+len(newCandles) {
		out = append([]market.Candle{}, base...)
	}
	for _, c := range newCandles {
		if len(out) > 0 && c.Time <= out[len(out)-1].Time {
			continue // duplicato o fuori ordine: scarta
		}
		out = append(out, c)
	}
	return out
}

// UpdateCSV aggiorna il file di cache con le candele mancanti e ritorna
// la serie completa.
func UpdateCSV(ctx context.Context, path string, c *Client, symbol, interval string) ([]market.Candle, error) {
	base, err := LoadCSV(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var start int64
	if len(base) > 0 {
		start = base[len(base)-1].Time + 1
	}
	fresh, err := c.FetchKlines(ctx, symbol, interval, start, 0)
	if err != nil {
		return base, err
	}
	merged := Merge(base, fresh)
	if err := SaveCSV(path, merged); err != nil {
		return merged, err
	}
	return merged, nil
}
