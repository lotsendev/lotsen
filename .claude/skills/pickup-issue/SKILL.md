---
name: pickup-issue
description: Reads a GitHub issue, determines whether it is frontend (React) or backend (Go) work, and hands off to the correct engineer skill. Use when the user says "pick up issue", "work on issue", "implement issue", or mentions an issue number without specifying frontend or backend.
---

# Pick Up Issue

## Workflow

### 1. Read the issue

Use `mcp__github__get_issue` with `owner=ercadev`, `repo=lotsen`, and the issue number.
Also call `mcp__github__get_issue_comments` to read any discussion.

Read the full body, acceptance criteria, and any comments.

### 2. Determine the domain

**Go (backend)** — issue involves any of:
- REST API handlers
- Data storage / state
- Docker orchestration
- Business logic
- systemd / installer

**React (frontend)** — issue involves any of:
- UI components or pages
- Forms or user interactions
- Real-time updates in the browser
- Styling or layout

**Both** — split the work: implement the Go changes first, then hand off to the React engineer for the UI layer.

### 3. Hand off to the right skill

- Backend only → invoke `go-engineer` skill
- Frontend only → invoke `react-engineer` skill
- Both → invoke `go-engineer` first, then `react-engineer`

Pass full issue context to the skill so it does not need to re-fetch.
