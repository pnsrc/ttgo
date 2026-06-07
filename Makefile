BINARY = trusttunnel_endpoint
CMD    = ./cmd/endpoint

.PHONY: build run lint tidy

build:
	go build -ldflags="-s -w" -o $(BINARY) $(CMD)

run:
	sudo ./$(BINARY) vpn.toml hosts.toml -l debug

tidy:
	go mod tidy

lint:
	golangci-lint run ./...
