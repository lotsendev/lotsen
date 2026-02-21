.PHONY: setup dev build test clean

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
	(cd orchestrator && DIRIGENT_DATA=/tmp/dirigent.json $(AIR)) & \
	(cd dashboard && bun run dev) & \
	wait

# Compile the Go binaries.
build:
	cd api && go build -o ../dirigent ./cmd/dirigent
	cd orchestrator && go build -o ../dirigent-orchestrator ./cmd/orchestrator

# Run the Go test suites.
test:
	cd api && go test ./...
	cd orchestrator && go test ./...

# Remove build artifacts.
clean:
	rm -f dirigent dirigent-orchestrator
	rm -rf api/tmp orchestrator/tmp
