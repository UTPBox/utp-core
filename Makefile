BINARY_NAME=utp-core
BUILD_DIR=build
VERSION := $(shell git describe --tags --always --dirty)
COMMIT := $(shell git rev-parse --short HEAD)

# Build flags
LDFLAGS=-s -w -checklinkname=0
# Removed with_ech and with_reality_server as they are deprecated/merged and cause build errors
TAGS=-tags "with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api,tfogo_checklinkname0"
BUILD_FLAGS=-ldflags "$(LDFLAGS) -X main.version=$(VERSION) -X main.commit=$(COMMIT)"

.PHONY: all build build-linux build-windows clean deps test help version

all: build-all

build: build-linux build-windows

build-linux:
	@echo "Building $(BINARY_NAME) for Linux..."
	GOOS=linux GOARCH=amd64 go build -v $(TAGS) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/utp-core

build-windows:
	@echo "Building $(BINARY_NAME) for Windows..."
	GOOS=windows GOARCH=amd64 go build -v $(TAGS) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/utp-core

build-all: build-linux build-windows

run:
	@go run ./cmd/utp-core run -c config.json

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)

deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

test:
	@go test ./...

version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build-linux    Build for Linux (amd64)"
	@echo "  build-windows  Build for Windows (amd64)"
	@echo "  build-all      Build for both Linux and Windows"
	@echo "  clean          Remove build artifacts"
	@echo "  deps           Download and tidy dependencies"
	@echo "  help           Show this help message"
