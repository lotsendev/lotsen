.PHONY: setup dev build test clean proxy

AIR := $(shell go env GOPATH)/bin/air

# Install development tools and dashboard dependencies.
setup:
	go install github.com/air-verse/air@latest
	cd dashboard && bun install

# Start the Go API (with Air hot reload) and the Vite dashboard dev server
# concurrently in a single terminal. Ctrl+C shuts down both processes cleanly.
dev:
	@trap 'kill 0' SIGINT; \
	(cd api && DIRIGENT_DATA=/tmp/dirigent.json $(AIR)) & \
	(cd orchestrator && DIRIGENT_DATA=/tmp/dirigent.json DIRIGENT_PROXY_HEALTH_URL=http://localhost:8090/internal/health $(AIR)) & \
	(cd proxy && DIRIGENT_DATA=/tmp/dirigent.json DIRIGENT_PROXY_ADDR=:8090 $(AIR)) & \
	(cd dashboard && bun run dev) & \
	wait

# Compile the Go binaries.
build:
	cd cli && go build -o ../dirigent-cli ./cmd/dirigent
	cd api && go build -o ../dirigent ./cmd/dirigent
	cd orchestrator && go build -o ../dirigent-orchestrator ./cmd/orchestrator
	cd proxy && go build -o ../dirigent-proxy ./cmd/proxy

# Run the Go test suites.
test:
	cd api && go test ./...
	cd orchestrator && go test ./...
	cd proxy && go test ./...

# Start the Vite dev server for the marketing website.
dev-website:
	cd website && bun run dev

# Remove build artifacts.
clean:
	rm -f dirigent-cli dirigent dirigent-orchestrator dirigent-proxy
	rm -rf api/tmp orchestrator/tmp proxy/tmp
