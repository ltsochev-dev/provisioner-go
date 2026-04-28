BINARY := provisioner
BUILD_DIR := bin
PACKAGE := ./cmd/provisioner

.PHONY: fmt test vet all build build-native build-windows-amd64 build-linux-amd64 build-linux-arm64 run

fmt:
	gofmt -w cmd internal

test:
	go test ./...

vet:
	go vet ./...

all: build-linux-amd64 build-linux-arm64 build-windows-amd64

build: build-linux-amd64 build-linux-arm64

build-native:
	go build -o $(BUILD_DIR)/$(BINARY)-native $(PACKAGE)

build-windows-amd64:
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe $(PACKAGE)

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY)-linux-amd64 $(PACKAGE)

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY)-linux-arm64 $(PACKAGE)

run:
	docker compose up -d
	go run ./cmd/provisioner
