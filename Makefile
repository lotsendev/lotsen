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
	(cd control-plane && DIRIGENT_DATA=/tmp/dirigent.json $(AIR)) & \
	(cd dashboard && bun run dev) & \
	wait

# Compile the Go binary to ./dirigent.
build:
	cd control-plane && go build -o ../dirigent ./cmd/dirigent

# Run the Go test suite.
test:
	cd control-plane && go test ./...

# Remove build artifacts.
clean:
	rm -f dirigent
	rm -rf control-plane/tmp
