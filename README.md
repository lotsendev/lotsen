# Dirigent

A lightweight Docker orchestration tool for solo developers and small teams running production workloads on a VPS — a simpler alternative to Kubernetes.

## Installation

Run the following command on a fresh Ubuntu 22.04+ or Debian 11+ VPS as root (or with `sudo`):

```bash
curl -fsSL https://github.com/ercadev/dirigent-releases/releases/latest/download/install.sh | sudo bash
```

To pin a specific version:

```bash
DIRIGENT_VERSION=v0.0.2 curl -fsSL https://github.com/ercadev/dirigent-releases/releases/download/v0.0.2/install.sh | sudo bash
```

The installer will:
- Install Docker Engine if not already present
- Install Bun if not already present
- Download all Dirigent components for your architecture (`amd64` / `arm64`)
- Register and start four systemd services that survive reboots
- Create the `/var/lib/dirigent/` data directory and the `dirigent` Docker network

Re-running the installer performs an in-place upgrade.

### Ports

| Service               | Port   | Description                                    |
|-----------------------|--------|------------------------------------------------|
| `dirigent-api`        | `:8080`| REST API — reads/writes deployment state       |
| `dirigent-orchestrator` | —    | Reconciler — syncs state with Docker (no port) |
| `dirigent-proxy`      | `:80`  | Reverse proxy — routes traffic to containers   |
| `dirigent-dashboard`  | `:3000`| Web dashboard                                  |

**Why is the dashboard on `:3000` and not behind the proxy at `:80`?**
The reverse proxy only routes traffic to your deployed containers. The dashboard must remain reachable before any containers are configured, so it runs as its own service on a dedicated port.

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
| `dashboard/`     | React + Vite web dashboard (`:3000`)               |

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
make build   # compile the Go binary → ./dirigent
make test    # run go test ./...
make clean   # remove build artifacts
```
