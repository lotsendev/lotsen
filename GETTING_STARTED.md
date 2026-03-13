# Getting Started with Lotsen

This guide covers two paths: running Lotsen on a VPS (production) and running it locally for development.

---

## Production — install on a VPS

### Prerequisites

- Ubuntu 22.04+ or Debian 11+
- A VPS with root access
- Ports 80 and 3000 open in your firewall

### Install

```bash
curl -fsSL https://github.com/lotsendev/lotsen/releases/latest/download/install.sh | sudo bash
sudo lotsen setup
```

The bootstrap installer installs the `lotsen` CLI.

Then `lotsen setup` will:
1. Install Docker Engine if not already present
2. Download all Lotsen components for your architecture (`amd64` or `arm64`)
3. Create and enable three systemd services
4. Print a summary of running services and their ports

In interactive mode, setup recommends the `strict` security profile.

### Services after install

| Service | Port | Description |
|---|---|---|
| `lotsen-api` | `:8080` | REST API + dashboard UI |
| `lotsen-proxy` | `:80` | Reverse proxy for your deployments |
| `lotsen-orchestrator` | — | Internal reconciler, no inbound port |

Open the dashboard at `http://<your-vps-ip>:8080`.

> **Why port 8080?** The API now serves the embedded dashboard bundle directly, so production no longer requires Bun or Node on the VPS. For HTTPS on a dedicated domain, run `sudo lotsen setup` and set dashboard exposure.

### Configure dashboard access after install

You can re-run dashboard exposure/auth setup at any time:

```bash
sudo lotsen setup
```

This command updates `/etc/lotsen/lotsen.env` and restarts `lotsen-proxy`.

### Pin a specific version

```bash
LOTSEN_VERSION=v0.1.0 curl -fsSL https://github.com/lotsendev/lotsen/releases/download/v0.1.0/install.sh | sudo bash
sudo LOTSEN_VERSION=v0.1.0 lotsen setup
```

### Upgrade

Use the dashboard upgrade flow: open **Settings** and click **Upgrade** when a newer version is available.
The dashboard streams installer logs in real time and prompts you to reload once the API is back online.

If dashboard access is unavailable, you can still run the manual installer command. The installer stops all services,
replaces the binaries, and restarts cleanly.

```bash
sudo lotsen upgrade
```

By default, `lotsen upgrade` now shows the current and target version, then asks for confirmation before continuing.
For unattended runs (for example CI/automation), pass `--non-interactive --yes`.
Upgrades refresh Lotsen binaries/services only and do not modify host firewall or SSH hardening settings.

### Manage services

```bash
# View logs for a specific service
journalctl -u lotsen-api -f
journalctl -u lotsen-orchestrator -f
journalctl -u lotsen-proxy -f

# Restart a service
sudo systemctl restart lotsen-api

# Check status of all services
sudo systemctl status 'lotsen-*'
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

All services share a single JSON file at `/tmp/lotsen.json` in dev mode. Delete it to reset state.

---

## Deploy your first container

1. Open the dashboard (`http://localhost:5173` in dev, or `http://<vps-ip>:8080` in production)
2. Click **New Deployment**
3. Fill in a name, Docker image, and any ports or environment variables
4. Click **Deploy**

The orchestrator picks up the new deployment within 15 seconds and starts the container. Status updates appear in real time on the dashboard.
