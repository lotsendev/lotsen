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

- **Backend:** Go (Docker orchestrator)
- **Frontend:** React (web GUI)
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

### Backend (Go)

```bash
# Run the server (serves gui/dist/ on :8080)
go run .

# Build a production binary
go build -o dirigent .

# Run tests
go test ./...
```

### Frontend (React + Vite)

```bash
cd gui

# Install dependencies (first time)
npm install

# Development server with API proxy to :8080
npm run dev

# Production build (outputs to gui/dist/)
npm run build
```

### Full production workflow

```bash
cd gui && npm run build && cd .. && go run .
```

## Architecture

```
/
├── main.go          Go HTTP server (:8080) — API routes + SPA static file serving
├── go.mod
└── gui/             Vite + React frontend
    ├── src/
    │   ├── main.tsx
    │   ├── App.tsx
    │   └── pages/   One file per page (DeploymentList, etc.)
    └── dist/        Production build output (git-ignored), served by Go
```

- In **development**: run `go run .` for the API and `npm run dev` in `gui/` for the UI. Vite proxies `/api/*` to `:8080`.
- In **production**: build the frontend (`npm run build`), then run `go run .` which serves `gui/dist/` as a SPA with an `index.html` fallback for client-side routes.
