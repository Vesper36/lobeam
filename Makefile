.PHONY: build server cli frontend test clean run docker-build docker-run release

VERSION ?= 1.0.0
LDFLAGS = -ldflags="-s -w -X main.version=$(VERSION)"

# Build everything
build: server cli

# Build server with embedded frontend
server: frontend
	go build $(LDFLAGS) -o dist/mimo ./cmd/mimo/

# Build CLI
cli:
	go build $(LDFLAGS) -o dist/mimo-cli ./cmd/mimo-cli/

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
	./dist/mimo

# Docker
docker-build:
	docker build -t mimo:$(VERSION) -t mimo:latest .

docker-run:
	docker run --rm -p 8080:8080 -v $(PWD)/data:/data mimo:latest

# Release binaries for all platforms
release: clean
	mkdir -p dist
	cd web && npm install --silent && npm run build && cd ..

	# Linux
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/mimo-linux-amd64 ./cmd/mimo/
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/mimo-linux-arm64 ./cmd/mimo/
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/mimo-cli-linux-amd64 ./cmd/mimo-cli/
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/mimo-cli-linux-arm64 ./cmd/mimo-cli/

	# macOS
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/mimo-darwin-amd64 ./cmd/mimo/
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/mimo-darwin-arm64 ./cmd/mimo/
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/mimo-cli-darwin-amd64 ./cmd/mimo-cli/
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/mimo-cli-darwin-arm64 ./cmd/mimo-cli/

	# Windows
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/mimo-windows-amd64.exe ./cmd/mimo/
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o dist/mimo-windows-arm64.exe ./cmd/mimo/
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/mimo-cli-windows-amd64.exe ./cmd/mimo-cli/
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o dist/mimo-cli-windows-arm64.exe ./cmd/mimo-cli/

	@echo "Release binaries built in dist/"
	@ls -la dist/
