BINARY  = trusttunnel_endpoint
ADMIN   = ttadmin
CMD     = ./cmd/endpoint
ADMCMD  = ./cmd/ttadmin

.PHONY: build build-admin build-all run lint tidy deploy-linux

build:
	go build -ldflags="-s -w" -o $(BINARY) $(CMD)

build-admin:
	go build -ldflags="-s -w" -o $(ADMIN) $(ADMCMD)

build-all: build build-admin

build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BINARY).linux $(CMD)
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(ADMIN).linux $(ADMCMD)

build-mac:
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(ADMIN).mac-arm $(ADMCMD)
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(ADMIN).mac-amd $(ADMCMD)

build-android:
	GOOS=android GOARCH=arm64 go build -ldflags="-s -w" -o $(ADMIN).android-arm64 $(ADMCMD)

run:
	sudo ./$(BINARY) vpn.toml hosts.toml -l debug

tidy:
	go mod tidy

lint:
	golangci-lint run ./...
