.PHONY: setup dev build test clean proxy release

AIR := $(shell go env GOPATH)/bin/air

# Install development tools and dashboard dependencies.
setup:
	go install github.com/air-verse/air@latest
	cd dashboard && bun install

# Start the Go API (with Air hot reload) and the Vite dashboard dev server
# concurrently in a single terminal. Ctrl+C shuts down both processes cleanly.
dev:
	@trap 'kill 0' SIGINT; \
	(cd api && LOTSEN_DATA=/tmp/lotsen.json LOTSEN_PROXY_ACCESS_LOG_DIR=/tmp/lotsen-proxy-logs LOTSEN_JWT_SECRET=dev-secret LOTSEN_REGISTRY_SECRET=dev-secret LOTSEN_RP_ID=localhost LOTSEN_RP_ORIGINS=http://localhost:5173 $(AIR)) & \
	(cd orchestrator && LOTSEN_DATA=/tmp/lotsen.json LOTSEN_REGISTRY_SECRET=dev-secret LOTSEN_PROXY_HEALTH_URL=http://localhost:8090/internal/health LOTSEN_PROXY_TRAFFIC_URL=http://localhost:8090/internal/traffic $(AIR)) & \
	(cd proxy && LOTSEN_DATA=/tmp/lotsen.json LOTSEN_PROXY_ADDR=:8090 LOTSEN_PROXY_HTTPS_ADDR=:8443 LOTSEN_CERT_CACHE_DIR=/tmp/lotsen-certs LOTSEN_PROXY_ACCESS_LOG_DIR=/tmp/lotsen-proxy-logs LOTSEN_JWT_SECRET=dev-secret $(AIR)) & \
	(cd dashboard && bun run dev) & \
	wait

# Compile the Go binaries.
build:
	cd dashboard && bun run build
	mkdir -p api/internal/dashboard/static
	cp -R dashboard/dist/. api/internal/dashboard/static/
	cd cli && go build -o ../lotsen-cli ./cmd/lotsen
	cd api && go build -o ../lotsen ./cmd/lotsen
	cd orchestrator && go build -o ../lotsen-orchestrator ./cmd/orchestrator
	cd proxy && go build -o ../lotsen-proxy ./cmd/proxy

# Run the Go test suites.
test:
	cd api && go test ./...
	cd orchestrator && go test ./...
	cd proxy && go test ./...

# Start the Vite dev server for the marketing website.
dev-website:
	cd website && bun run dev

# Trigger a release via conventional-commit analysis (requires gh CLI).
release:
	gh workflow run auto-tag.yml --repo lotsendev/lotsen

# Remove build artifacts.
clean:
	rm -f lotsen-cli lotsen lotsen-orchestrator lotsen-proxy
	rm -rf api/tmp orchestrator/tmp proxy/tmp
