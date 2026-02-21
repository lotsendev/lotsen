#!/usr/bin/env bash
# Usage: fetch-pr.sh [pr-number]
# Fetches PR metadata, changed files, and diff in a structured format.
set -euo pipefail

PR="${1:-}"

if [[ -z "$PR" ]]; then
  PR=$(gh pr view --json number -q '.number' 2>/dev/null || echo "")
fi

if [[ -z "$PR" ]]; then
  echo "Error: no PR number provided and no open PR found for current branch" >&2
  exit 1
fi

echo "=== PR METADATA ==="
gh pr view "$PR" --json number,title,author,state,baseRefName,headRefName,body,labels,additions,deletions,changedFiles

echo ""
echo "=== CHANGED FILES ==="
gh pr view "$PR" --json files -q '.files[].path'

echo ""
echo "=== DIFF ==="
gh pr diff "$PR"
