# CLAUDE.md - Lotsen

## Project Context
Docker orchestration for solo devs/VPS. Lightweight K8s alternative.
- **Repo:** `lotsendev/lotsen` (Always use for `gh` commands).
- **Stack:** Go (Backend/Orchestrator), Bun + React + Vite (Frontend).
- **Data Flow:** Dashboard → API → JSON Store ← Orchestrator → Docker.

## Development Commands
- **Full Stack:** `make dev` (API :8080, Dashboard :5173, Orchestrator).
- **Setup:** `make setup` | **Build:** `make build` | **Test:** `make test`.
- **Dashboard Prod:** bundled into `lotsen-api` via embedded static assets.

## Guidelines
### Git & Commits
Follow [Conventional Commits](https://www.conventionalcommits.org/).
- **Pattern:** `<type>(<scope>): <description>` (lowercase, imperative).
- **Types:** `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `perf`, `ci`.
- **Branching:** `main` is protected. Use `feat/`, `fix/`, etc. branches.

### Coding Standards
- **Backend:** Go 1.23. Use `internal/` for shared logic.
- **Frontend:** React with Vite. Pages go in `dashboard/src/pages/`.
- **State:** Shared via `/tmp/lotsen.json` in dev.
