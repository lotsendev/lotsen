# Getting Started with Dirigent

This guide covers two paths: running Dirigent on a VPS (production) and running it locally for development.

---

## Production — install on a VPS

### Prerequisites

- Ubuntu 22.04+ or Debian 11+
- A VPS with root access
- Ports 80 and 3000 open in your firewall

### Install

```bash
curl -fsSL https://github.com/ercadev/dirigent-releases/releases/latest/download/install.sh | sudo bash
sudo dirigent setup
```

The bootstrap installer installs the `dirigent` CLI.

Then `dirigent setup` will:
1. Install Docker Engine if not already present
2. Install Bun if not already present
3. Download all Dirigent components for your architecture (`amd64` or `arm64`)
4. Create and enable four systemd services
5. Print a summary of running services and their ports

In interactive mode, setup recommends the `strict` security profile.

### Services after install

| Service | Port | Description |
|---|---|---|
| `dirigent-api` | `:8080` | REST API |
| `dirigent-proxy` | `:80` | Reverse proxy for your deployments |
| `dirigent-dashboard` | `:3000` | Web UI |
| `dirigent-orchestrator` | — | Internal reconciler, no inbound port |

Open the dashboard at `http://<your-vps-ip>:3000`.

> **Why port 3000?** The dashboard runs on its own port rather than through the Dirigent proxy. This gives you immediate access without any DNS configuration and keeps future dashboard authentication independent of the proxy layer. To serve the dashboard on a custom domain, place a reverse proxy (nginx, Caddy, etc.) in front of port 3000.

### Pin a specific version

```bash
DIRIGENT_VERSION=v0.1.0 curl -fsSL https://github.com/ercadev/dirigent-releases/releases/download/v0.1.0/install.sh | sudo bash
sudo DIRIGENT_VERSION=v0.1.0 dirigent setup
```

### Upgrade

Re-run the same install command. The installer stops all services, replaces the binaries, and restarts cleanly.

### Manage services

```bash
# View logs for a specific service
journalctl -u dirigent-api -f
journalctl -u dirigent-orchestrator -f
journalctl -u dirigent-proxy -f
journalctl -u dirigent-dashboard -f

# Restart a service
sudo systemctl restart dirigent-api

# Check status of all services
sudo systemctl status 'dirigent-*'
```

---

## Local development

### Prerequisites

| Tool | Version | Install |
|---|---|---|
| Go | 1.23+ | https://go.dev/dl/ |
| Bun | 1.x | https://bun.sh |
| Air | latest | `go install github.com/air-verse/air@latest` |
| Docker Desktop | latest | https://www.docker.com/products/docker-desktop |

### First-time setup

```bash
make setup
```

Installs Air and all dashboard dependencies.

### Start the full stack

```bash
make dev
```

Starts all four components with hot reload in a single terminal:

| Component | URL |
|---|---|
| API | `http://localhost:8080` |
| Dashboard | `http://localhost:5173` (Vite dev server) |
| Proxy | `http://localhost:8090` |
| Orchestrator | — |

Vite proxies `/api/*` to the API at `:8080` automatically. Press **Ctrl+C** to stop everything.

### Other make targets

```bash
make build   # compile all three Go binaries
make test    # run all Go test suites
make clean   # remove build artifacts
```

### State

All services share a single JSON file at `/tmp/dirigent.json` in dev mode. Delete it to reset state.

---

## Deploy your first container

1. Open the dashboard (`http://localhost:5173` in dev, or `http://<vps-ip>:3000` in production)
2. Click **New Deployment**
3. Fill in a name, Docker image, and any ports or environment variables
4. Click **Deploy**

The orchestrator picks up the new deployment within 15 seconds and starts the container. Status updates appear in real time on the dashboard.
