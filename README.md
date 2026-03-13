# Lotsen

A lightweight Docker orchestration tool for solo developers and small teams running production workloads on a VPS — a simpler alternative to Kubernetes.

## Installation

Run the following command on a fresh Ubuntu 22.04+ or Debian 11+ VPS as root (or with `sudo`):

```bash
curl -fsSL https://github.com/ercadev/lotsen-releases/releases/latest/download/install.sh | sudo bash
sudo lotsen setup
```

To pin a specific version:

```bash
LOTSEN_VERSION=v0.0.2 curl -fsSL https://github.com/ercadev/lotsen-releases/releases/download/v0.0.2/install.sh | sudo bash
sudo LOTSEN_VERSION=v0.0.2 lotsen setup
```

The bootstrap installer will:
- Install the `lotsen` CLI binary

Then `lotsen setup` will:
- Install Docker Engine if not already present
- Download all Lotsen components for your architecture (`amd64` / `arm64`)
- Register and start three systemd services that survive reboots
- Create the `/var/lib/lotsen/` data directory and the `lotsen` Docker network
- Offer security profiles in guided mode (`strict` is recommended)
- Configure proxy hardening profiles (`standard` by default, `strict` recommended for internet-facing hosts)
- Prompt for initial dashboard `/login` credentials in interactive setup (blank password auto-generates one)
- Configure passkey relying-party settings automatically when `LOTSEN_DASHBOARD_DOMAIN` is set

Re-running the installer performs an in-place upgrade.

### Proxy hardening profiles

Lotsen proxy supports three hardening levels:

- `standard` (default): blocks sensitive file and dot-path probes like `/.env`, `/.git`, and `/.vscode`
- `strict`: includes `standard` plus broader scanner-target blocks (`/swagger*`, `/actuator`, `/wp-*`, etc.) and tighter anti-scan throttling
- `off`: disables proxy hardening checks

Configure during setup:

```bash
sudo lotsen setup --proxy-hardening-profile strict
```

Or via environment variable:

```bash
sudo LOTSEN_PROXY_HARDENING_PROFILE=strict lotsen setup
```

### Ports

| Service               | Port   | Description                                    |
|-----------------------|--------|------------------------------------------------|
| `lotsen-api`          | `:8080`| REST API + dashboard UI                        |
| `lotsen-orchestrator` | —      | Reconciler — syncs state with Docker (no port) |
| `lotsen-proxy`        | `:80`  | Reverse proxy — routes traffic to containers   |

The dashboard is served by `lotsen-api` on `:8080` by default. If you set `LOTSEN_DASHBOARD_DOMAIN` during setup, the proxy exposes it on `:80/:443` over HTTPS.

`LOTSEN_AUTH_USER`/`LOTSEN_AUTH_PASSWORD` are bootstrap-only: they seed the first dashboard user when `users.db` is empty and are ignored after users already exist.

Set `LOTSEN_AUTH_COOKIE_DOMAIN` to enable shared dashboard/deployment auth on subdomains of the same parent domain (for example `d0001.erca.dev`).

## Features

- One-script installer, up and running fast
- Web dashboard to deploy, edit, and remove Docker containers
- GitOps-based deployments as an alternative workflow
- Zero-downtime rolling deployments
- Integrated load balancer / reverse proxy

## Why?

- Managing Docker containers on a VPS today is painful
- Kubernetes is overkill and expensive for solo developers and small teams

## Monorepo structure

| Directory        | Description                                        |
|------------------|----------------------------------------------------|
| `api/`           | Go REST API — reads/writes the JSON store (`:8080`) |
| `orchestrator/`  | Go reconciler — syncs store state with Docker      |
| `store/`         | Shared Go module — deployment types + JSON store   |
| `dashboard/`     | React + Vite web dashboard (dev server `:5173`)    |

The three Go services share a single `go.work` workspace at the repo root.

## Tech stack

- **api / orchestrator / store:** Go
- **dashboard:** React, Vite, Bun

## Local development

### Prerequisites

| Tool           | Version  | Install                                      |
|----------------|----------|----------------------------------------------|
| Go             | 1.23+    | https://go.dev/dl/                           |
| Bun            | 1.x      | https://bun.sh                               |
| Air            | latest   | `go install github.com/air-verse/air@latest` |
| Docker Desktop | latest   | https://www.docker.com/products/docker-desktop |

### First-time setup

```bash
make setup
```

Installs Air and dashboard dependencies in one step.

### Start the full stack

```bash
make dev
```

Starts both the Go API (Air hot reload on `:8080`) and the Vite dashboard dev server (`:5173`) in a single terminal. Vite proxies `/api/*` to `:8080`.

- Saving a `.go` file recompiles and restarts the API automatically.
- Saving a `.tsx` / `.ts` file hot-reloads the browser without a full page refresh.
- Press **Ctrl+C** to shut down both processes cleanly.

### Other targets

```bash
make build   # compile the Go binary → ./lotsen
make test    # run go test ./...
make clean   # remove build artifacts
```

## Release workflow secrets

The release workflow (`.github/workflows/release.yml`) publishes the landing page Docker image to Docker Hub on `v*` tags.

Configure these repository secrets before creating a release tag:

- `DOCKERHUB_USERNAME`
- `DOCKERHUB_TOKEN`
