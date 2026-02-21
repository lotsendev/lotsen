---
name: review-pr
description: Reviews GitHub pull requests by fetching PR diffs, summarizing changes, checking for security, style, test coverage, and conventional commit issues, then posting structured feedback as GitHub review comments via the gh CLI. Use when the user asks to review a PR, check a pull request, give feedback on GitHub changes, or mentions a PR number.
---

# PR Review

## Quick start

```
/review-pr 42    # Review PR #42
/review-pr       # Review the PR for the current branch
```

## Workflow

1. **Fetch PR data** — run `scripts/fetch-pr.sh [number]` to get metadata, diff, and changed files
2. **Summarize** — write a plain-English summary of what the PR does and why
3. **Check for issues** — work through the checklist below
4. **Draft feedback** — group findings by file or theme; separate critical from minor
5. **Post review** — submit via `gh pr review` (see commands below)

## Review checklist

- [ ] **Correctness** — does the logic do what it claims?
- [ ] **Security** — input validation, injection risks, no secrets committed, OWASP top 10
- [ ] **Tests** — new code paths covered; edge cases tested
- [ ] **Style** — consistent with surrounding code; no dead code or debug artifacts
- [ ] **Conventional commits** — PR title and commits follow `type(scope): description`
- [ ] **Breaking changes** — API/contract changes flagged with `!`?
- [ ] **Docs** — does anything need a README or comment update?

## Posting to GitHub

### Approve
```bash
gh pr review <number> --approve --body "<summary>"
```

### Request changes
```bash
gh pr review <number> --request-changes --body "<summary>"
```

### Comment only
```bash
gh pr review <number> --comment --body "<summary>"
```

### Inline file comment
```bash
gh api repos/{owner}/{repo}/pulls/<number>/comments \
  --method POST \
  -f body="<comment>" \
  -f path="<file>" \
  -f line=<line>
```

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
