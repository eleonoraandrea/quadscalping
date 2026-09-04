BINARY := hpsbot
BUILD_DIR := build

.PHONY: build test fmt vet lint backtest download clean

build:
	go build -o $(BUILD_DIR)/hpsbot ./cmd/hpsbot
	go build -o $(BUILD_DIR)/backtest ./cmd/backtest
	go build -o $(BUILD_DIR)/download ./cmd/download

test:
	go test ./... -race

fmt:
	gofmt -l -w .

vet:
	go vet ./...

lint: fmt vet test

# backtest di default: BTCUSDT 5m, report in reports/report.html
backtest:
	go run ./cmd/backtest -config config.json -out reports/report.html

download:
	go run ./cmd/download -symbol BTCUSDT -interval 5m

clean:
	rm -rf $(BUILD_DIR)
