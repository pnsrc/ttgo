BINARY  = trusttunnel_endpoint
ADMIN   = ttadmin
CLIENT  = ttclient
CMD     = ./cmd/endpoint
ADMCMD  = ./cmd/ttadmin
CLIENTCMD = ./cmd/ttclient

.PHONY: build build-admin build-client build-all run lint tidy deploy-linux client

build:
	go build -ldflags="-s -w" -o $(BINARY) $(CMD)

build-admin:
	go build -ldflags="-s -w" -o $(ADMIN) $(ADMCMD)

build-client:
	cd cmd/ttclient && wails build

build-client-dev:
	cd cmd/ttclient && wails build -devtools

client-dev:
	cd cmd/ttclient && wails dev

client: build-client
	sudo ./cmd/ttclient/build/bin/ttclient.app/Contents/MacOS/ttclient

build-all: build build-admin build-client

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
