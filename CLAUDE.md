# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## GitHub Repository

**Repo:** `ercadev/dirigent`

Always use `ercadev/dirigent` when calling `gh` CLI commands (e.g. `gh issue view 6 --repo ercadev/dirigent`). Never use `erikcarlsson/dirigent`.

## Project Overview

Dirigent is a Docker container orchestration tool for solo developers and small teams running production workloads on a VPS. It is positioned as a lightweight alternative to Kubernetes, offering:

- One-script installer
- Web GUI for deploying/editing/removing Docker containers
- GitOps-based deployment alternative
- Zero-downtime Docker deployments
- Integrated load balancer / reverse proxy

## Tech Stack

- **Backend:** Go (Docker orchestrator + REST API)
- **Frontend:** Bun + React + Vite (dashboard, runs as a Docker container)
- **Infrastructure:** Docker, VPS

## Git Conventions

All commits in this repository must follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(optional scope): <short description>

[optional body]
```

**Types:**

| Type | When to use |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `refactor` | Code change that is neither a fix nor a feature |
| `test` | Adding or correcting tests |
| `chore` | Build process, tooling, dependency updates |
| `perf` | Performance improvement |
| `ci` | CI/CD configuration |

**Branch policy:**
- `main` is protected — never commit directly to it
- All work must happen on a feature branch: `git checkout -b feat/short-description`
- Branch names should mirror the conventional commit type: `feat/`, `fix/`, `chore/`, etc.
- Merge to `main` only via pull request

**Commit rules:**
- Description is lowercase, imperative mood, no trailing period
- Scope is optional but recommended for larger codebases (e.g. `feat(auth): ...`)
- Breaking changes append `!` after the type: `feat!: remove legacy endpoint`
- Body is free-form and used to explain *why*, not *what*

**Examples:**
```
feat(deploy): add zero-downtime rolling restart
fix(proxy): correctly forward X-Forwarded-For header
chore: upgrade Go to 1.23
docs: add architecture overview to CLAUDE.md
```

## Build and Run

### Local development (Makefile)

```bash
# First-time setup: install Air and dashboard dependencies
make setup

# Start the full stack (Go API + Vite dashboard) in one terminal
# Ctrl+C shuts down both processes cleanly
make dev

# Compile the Go binary → ./dirigent
make build

# Run Go tests
make test

# Remove build artifacts
make clean
```

`make dev` runs the Go API via Air (hot reload on `:8080`), the orchestrator via Air, and the Vite dev server on `:5173`. Vite proxies `/api/*` to `:8080`. Both Go processes share state via `/tmp/dirigent.json` in dev mode.

### Dashboard (production build)

```bash
cd dashboard

# Production build (outputs to dashboard/dist/)
bun run build

# Run the production server (serves dist/ on :3000)
bun run start
```

### Docker (production)

```bash
# Build the dashboard image
docker build -t dirigent-dashboard ./dashboard

# The dashboard container is managed by Dirigent in production
```

## Architecture

```
/
├── api/             Go REST API — reads/writes the JSON store (:8080)
│   ├── cmd/
│   │   └── dirigent/
│   ├── internal/    Shared Go packages (api handlers, store)
│   └── go.mod
├── orchestrator/    Go reconciler — syncs store state with Docker
│   ├── cmd/
│   │   └── orchestrator/
│   ├── internal/    docker/, store/, reconciler/ packages
│   └── go.mod
├── dashboard/       Bun + React + Vite dashboard (ships as Docker image)
│   ├── src/
│   │   ├── main.tsx
│   │   ├── App.tsx
│   │   └── pages/   One file per page
│   ├── server.ts    Bun SPA server for production (:3000)
│   ├── Dockerfile
│   └── dist/        Production build output (git-ignored)
```

Data flow: **dashboard → api → (json store) ← orchestrator → docker**

- In **development**: run `go run ./cmd/dirigent` inside `api/` for the REST API and `go run ./cmd/orchestrator` inside `orchestrator/` for the reconciler, then `bun run dev` inside `dashboard/`. Vite proxies `/api/*` to `:8080`.
- In **production**: the dashboard runs as a Docker container (managed by Dirigent) serving the Vite build via `server.ts` on `:3000`.
