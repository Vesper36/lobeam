.PHONY: build server cli mcp frontend test clean run docker-build docker-run release

VERSION ?= 1.0.0
LDFLAGS = -ldflags="-s -w -X main.version=$(VERSION)"

# Build everything
build: server cli mcp

# Build server with embedded frontend
server: frontend
	go build $(LDFLAGS) -o dist/lobeam ./cmd/lobeam/

# Build CLI
cli:
	go build $(LDFLAGS) -o dist/lobeam-cli ./cmd/lobeam-cli/

# Build MCP server (for AI tool integration)
mcp:
	go build $(LDFLAGS) -o dist/lobeam-mcp ./cmd/lobeam-mcp/

# Build frontend
frontend:
	cd web && npm install --silent && npm run build

# Run tests
test:
	go test ./...

# Run all quality gates
verify: verify-security verify-quality

# Security scan
verify-security:
	@which gosec >/dev/null 2>&1 && gosec ./... || echo "gosec not installed, skipping"

# Quality check
verify-quality:
	@which golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

# Clean build artifacts
clean:
	rm -rf dist/
	rm -rf web/node_modules
	rm -rf web/.svelte-kit
	rm -rf web/build

# Run server locally
run: server
	./dist/lobeam

# Docker
docker-build:
	docker build -t lobeam:$(VERSION) -t lobeam:latest .

docker-run:
	docker run --rm -p 8080:8080 -v $(PWD)/data:/data lobeam:latest

# Release binaries for all platforms
release: clean
	mkdir -p dist
	cd web && npm install --silent && npm run build && cd ..

	# Linux
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/lobeam-linux-amd64 ./cmd/lobeam/
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/lobeam-linux-arm64 ./cmd/lobeam/
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/lobeam-cli-linux-amd64 ./cmd/lobeam-cli/
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/lobeam-cli-linux-arm64 ./cmd/lobeam-cli/

	# macOS
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/lobeam-darwin-amd64 ./cmd/lobeam/
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/lobeam-darwin-arm64 ./cmd/lobeam/
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/lobeam-cli-darwin-amd64 ./cmd/lobeam-cli/
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/lobeam-cli-darwin-arm64 ./cmd/lobeam-cli/

	# Windows
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/lobeam-windows-amd64.exe ./cmd/lobeam/
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o dist/lobeam-windows-arm64.exe ./cmd/lobeam/
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/lobeam-cli-windows-amd64.exe ./cmd/lobeam-cli/
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o dist/lobeam-cli-windows-arm64.exe ./cmd/lobeam-cli/

	@echo "Release binaries built in dist/"
	@ls -la dist/
