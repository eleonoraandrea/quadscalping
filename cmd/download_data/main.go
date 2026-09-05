package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Kline []interface{}

func downloadKlines(symbol, interval string, startTime, endTime int64) ([]Kline, error) {
	var allKlines []Kline
	limit := 1000

	for start := startTime; start < endTime; start += int64(limit) * 60000 { // 60000ms = 1 minuto per M1
		url := fmt.Sprintf("https://api.binance.com/api/v3/klines?symbol=%s&interval=%s&startTime=%d&endTime=%d&limit=%d",
			symbol, interval, start, start+int64(limit)*60000, limit)

		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}

		var klines []Kline
		if err := json.NewDecoder(resp.Body).Decode(&klines); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		if len(klines) == 0 {
			break
		}

		allKlines = append(allKlines, klines...)
		fmt.Printf("Scaricate %d candele M1 per %s...\n", len(klines), symbol)
		time.Sleep(200 * time.Millisecond)
	}

	return allKlines, nil
}

func saveToCSV(filename string, klines []Kline) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	writer.Write([]string{"time", "open", "high", "low", "close", "volume"})

	for _, k := range klines {
		record := make([]string, 6)
		for i := 0; i < 6; i++ {
			switch v := k[i].(type) {
			case string:
				record[i] = v
			case float64:
				record[i] = fmt.Sprintf("%.8f", v)
			default:
				record[i] = fmt.Sprintf("%v", v)
			}
		}
		writer.Write(record)
	}
	return nil
}

func aggregateData(filename string, m1Klines []Kline) {
	// Aggrega M1 in M15, H1, H4
	timeframes := map[string]int{
		"15m": 15,
		"1h":  60,
		"4h":  240,
	}

	for tf, minutes := range timeframes {
		aggregated := aggregateKlines(m1Klines, minutes)
		tfFilename := fmt.Sprintf("data/%s_%s.csv", extractSymbol(filename), tf)
		if err := saveToCSV(tfFilename, aggregated); err != nil {
			fmt.Printf("Errore salvataggio %s: %v\n", tfFilename, err)
			continue
		}
		fmt.Printf("✓ Salvato %s (%d candele %s)\n", tfFilename, len(aggregated), tf)
	}
}

func aggregateKlines(m1Klines []Kline, minutes int) []Kline {
	if len(m1Klines) == 0 {
		return []Kline{}
	}

	var aggregated []Kline
	var currentGroup []Kline
	groupStart := -1

	for i, k := range m1Klines {
		timestamp := int64(k[0].(float64))
		currentMinute := int(timestamp / 60000)

		if groupStart == -1 {
			groupStart = currentMinute / minutes
		}

		currentGroupIdx := currentMinute / minutes

		if currentGroupIdx == groupStart {
			currentGroup = append(currentGroup, k)
		} else {
			// Aggrega il gruppo corrente
			if len(currentGroup) > 0 {
				agg := aggregateSingleGroup(currentGroup)
				aggregated = append(aggregated, agg)
			}
			currentGroup = []Kline{k}
			groupStart = currentGroupIdx
		}

		// Ultimo elemento
		if i == len(m1Klines)-1 && len(currentGroup) > 0 {
			agg := aggregateSingleGroup(currentGroup)
			aggregated = append(aggregated, agg)
		}
	}

	return aggregated
}

func aggregateSingleGroup(group []Kline) Kline {
	if len(group) == 0 {
		return Kline{}
	}

	open := group[0][1]
	high := group[0][2]
	low := group[0][3]
	close := group[len(group)-1][4]
	
	var volume float64
	for _, k := range group {
		vol, _ := strconv.ParseFloat(k[5].(string), 64)
		volume += vol
		
		// Aggiorna high/low
		h, _ := strconv.ParseFloat(k[2].(string), 64)
		l, _ := strconv.ParseFloat(k[3].(string), 64)
		
		if hHigh, _ := strconv.ParseFloat(high.(string), 64); h > hHigh {
			high = k[2]
		}
		if lLow, _ := strconv.ParseFloat(low.(string), 64); l < lLow {
			low = k[3]
		}
	}

	// Volume totale come stringa
	volumeStr := fmt.Sprintf("%.8f", volume)
	
	// Timestamp come intero (Binance usa millisecondi)
	timestamp := fmt.Sprintf("%.0f", group[0][0].(float64))

	return Kline{timestamp, open, high, low, close, volumeStr}
}

func extractSymbol(filename string) string {
	// Estrae il simbolo dal filename (es. "BTCUSDT_1m.csv" -> "BTCUSDT")
	start := 5 // dopo "data/"
	end := 0
	for i, c := range filename[start:] {
		if c == '_' {
			end = start + i
			break
		}
	}
	if end == 0 {
		end = len(filename) - 4 // rimuovi ".csv"
	}
	return filename[start:end]
}

func main() {
	symbols := []string{"BTCUSDT", "ETHUSDT", "DOGEUSDT"}
	interval := "1m" // Scarichiamo M1 per aggregare a tutti i TF successivi

	// Scarichiamo 3 mesi di dati M1 (circa 130k candele per simbolo)
	// Binance permette di scaricare fino a 1000 candele per richiesta
	endTime := time.Now().UnixMilli()
	startTime := endTime - int64(90*24*3600000) // 90 giorni

	fmt.Println("Download dati storici M1 da Binance (3 mesi)...")
	fmt.Printf("Periodo: %s - %s\n", time.UnixMilli(startTime).Format("2006-01-02"), time.UnixMilli(endTime).Format("2006-01-02"))
	fmt.Println("Questi dati M1 verranno usati per backtest accurati su M15, H1, H4")

	for _, symbol := range symbols {
		fmt.Printf("\nDownloading %s M1 data...\n", symbol)
		klines, err := downloadKlines(symbol, interval, startTime, endTime)
		if err != nil {
			fmt.Printf("Errore %s: %v\n", symbol, err)
			continue
		}

		filename := fmt.Sprintf("data/%s_1m.csv", symbol)
		if err := saveToCSV(filename, klines); err != nil {
			fmt.Printf("Errore salvataggio %s: %v\n", symbol, err)
			continue
		}

		fmt.Printf("✓ Salvato %s (%d candele M1)\n", filename, len(klines))
		
		// Aggrega automaticamente in M15, H1, H4
		fmt.Printf("Aggregazione dati per %s...\n", symbol)
		aggregateData(filename, klines)
	}

	fmt.Println("\n✅ Download e aggregazione completati!")
	fmt.Println("Dati disponibili:")
	fmt.Println("  - M1 (raw)")
	fmt.Println("  - M15 (aggregato)")
	fmt.Println("  - H1 (aggregato)")
	fmt.Println("  - H4 (aggregato)")
}
