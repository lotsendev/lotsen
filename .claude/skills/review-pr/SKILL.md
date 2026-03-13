---
name: review-pr
description: Reviews GitHub pull requests by fetching PR diffs, summarizing changes, checking for security, style, test coverage, and conventional commit issues, then posting structured feedback as GitHub review comments via the GitHub MCP. Use when the user asks to review a PR, check a pull request, give feedback on GitHub changes, or mentions a PR number.
---

# PR Review

## Quick start

```
/review-pr 42    # Review PR #42
/review-pr       # Review the PR for the current branch
```

## Workflow

1. **Fetch PR data** — use the GitHub MCP tools to get metadata, diff, and changed files:
   - `mcp__github__get_pull_request` — title, body, author, state, base/head branches, labels, additions/deletions
   - `mcp__github__get_pull_request_files` — list of changed files
   - `mcp__github__get_pull_request_diff` — full diff
2. **Summarize** — write a plain-English summary of what the PR does and why
3. **Check for issues** — work through the checklist below
4. **Draft feedback** — group findings by file or theme; separate critical from minor
5. **Post review** — submit via the GitHub MCP tools (see commands below)

## Review checklist

- [ ] **Correctness** — does the logic do what it claims?
- [ ] **Security** — input validation, injection risks, no secrets committed, OWASP top 10
- [ ] **Tests** — new code paths covered; edge cases tested
- [ ] **Style** — consistent with surrounding code; no dead code or debug artifacts
- [ ] **Conventional commits** — PR title and commits follow `type(scope): description`
- [ ] **Breaking changes** — API/contract changes flagged with `!`?
- [ ] **Docs** — does anything need a README or comment update?

## Posting to GitHub

Use `mcp__github__create_pull_request_review` with `owner=ercadev`, `repo=lotsen`, and the PR number.

### Approve
Set `event=APPROVE` and `body="<summary>"`.

### Request changes
Set `event=REQUEST_CHANGES` and `body="<summary>"`.

### Comment only
Set `event=COMMENT` and `body="<summary>"`.

### With inline comments
1. `mcp__github__create_pending_pull_request_review` — open a pending review
2. `mcp__github__add_pull_request_review_comment_to_pending_review` — add each inline comment (`path`, `line`, `body`)
3. `mcp__github__submit_pending_pull_request_review` — submit with `event` and top-level `body`

## Output format

Always structure the review as:

```
## Summary
<2-3 sentence plain-English description of what this PR does>

## Verdict
APPROVE | REQUEST_CHANGES | COMMENT

## Issues

### Critical
- `<file>:<line>` — <description>

### Minor
- `<file>:<line>` — <description>

## Nitpicks
- <optional style or docs suggestions>
```
