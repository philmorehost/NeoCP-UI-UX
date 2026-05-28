# NeoCP Professional - Cross-Platform Build System
# Final Binary: monolithic zero-dependency Go binary

BINARY_NAME=neocp
OUT_DIR=dist
CMD_DIR=neocp/cmd/neocp
WEB_DIR=neocp/web/static

# Linker flags: -s (disable symbol table), -w (disable DWARF generation) for smaller binaries
LDFLAGS=-ldflags="-s -w"

.PHONY: all build clean linux windows arm setup

all: clean setup linux windows

setup:
	@echo "Preparing embed patterns..."
	mkdir -p $(CMD_DIR)/web/static
	cp -r $(WEB_DIR)/* $(CMD_DIR)/web/static/

linux: setup
	@echo "Compiling for Linux AMD64..."
	cd neocp && GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o ../$(OUT_DIR)/$(BINARY_NAME)-linux-amd64 cmd/neocp/main.go

linux-arm: setup
	@echo "Compiling for Linux ARM64 (Graviton/Ampere)..."
	cd neocp && GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o ../$(OUT_DIR)/$(BINARY_NAME)-linux-arm64 cmd/neocp/main.go

windows: setup
	@echo "Compiling for Windows AMD64..."
	cd neocp && GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o ../$(OUT_DIR)/$(BINARY_NAME).exe cmd/neocp/main.go

clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(OUT_DIR)
	rm -rf $(CMD_DIR)/web
	rm -f $(BINARY_NAME)-linux-amd64
	rm -f $(BINARY_NAME).exe

test:
	@echo "Running core security and unit tests..."
	cd neocp && go test ./internal/...
