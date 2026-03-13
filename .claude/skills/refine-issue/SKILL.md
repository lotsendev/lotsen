---
name: refine-issue
description: Takes a rough idea, bug report, or vague feature request and turns it into a well-structured GitHub issue, then posts it via the gh CLI. Use when the user wants to create an issue, open an issue, report a bug, or says "refine issue", "write an issue", or describes something they want tracked on GitHub.
---

# Refine Issue

## Quick start

```
/refine-issue    # Describe your idea/bug and the skill handles the rest
```

## Workflow

### 1. Gather context

Ask the user:
- What's the problem or feature? (1-2 sentences)
- Is this a **bug**, **feature**, or **chore/task**?
- Which part of the system is affected? (`Go backend`, `React frontend`, `both`, `infra/config`)
- Any acceptance criteria or constraints already in mind?

If the user's initial message already answers these, skip straight to drafting.

### 2. Draft the issue

Pick the matching template below. Present the draft to the user and ask:
- Does this capture the intent correctly?
- Is anything missing or wrong?
- Should the title be adjusted?

Iterate until the user approves.

### 3. Post to GitHub

```bash
gh issue create \
  --repo ercadev/lotsen \
  --title "<title>" \
  --body "<body>"
```

Print the created issue URL when done.

---

## Issue templates

### Feature

```markdown
## What

<One-paragraph description of the feature and its user value.>

## Why

<Problem this solves or opportunity it creates.>

## Acceptance criteria

- [ ] <Criterion 1>
- [ ] <Criterion 2>

## Out of scope

<What this issue explicitly does NOT cover.>

## Discipline

**<Go (backend) | React (frontend) | Full-stack>**
```

### Bug

```markdown
## What's happening

<Brief description of the broken behavior.>

## Steps to reproduce

1. <Step 1>
2. <Step 2>

## Expected behavior

<What should happen.>

## Actual behavior

<What actually happens.>

## Discipline

**<Go (backend) | React (frontend) | Full-stack>**
```

### Chore / task

```markdown
## What

<What needs to be done and why.>

## Done when

- [ ] <Completion criterion>

## Discipline

**<Go (backend) | React (frontend) | Infra>**
```
