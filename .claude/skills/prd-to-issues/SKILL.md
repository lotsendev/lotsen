---
name: prd-to-issues
description: Product manages breaking down a PRD to issues
---

# PRD to Issues

Break a PRD into independently-grabbable GitHub issues using vertical slices (tracer bullets).

## Process

### 1. Locate the PRD

Ask the user for the PRD GitHub issue number (or URL). Fetch it with `gh issue view <number>`. Read and internalize the full PRD content (with all comments).

### 2. Explore the codebase

Read the key modules and integration layers referenced in the PRD. Identify:

- The distinct integration layers the feature touches (e.g. DB/schema, API/backend, UI, tests, config)
- Existing patterns for similar features
- Natural seams where work can be parallelized
- **Which disciplines are involved**: determine whether the PRD requires a Go engineer, a React engineer, or both

### 3. Draft vertical slices

Break the PRD into **tracer bullet** issues. Each issue is a thin vertical slice that cuts through ALL integration layers end-to-end, NOT a horizontal slice of one layer.

<vertical-slice-rules>
- Each slice delivers a narrow but COMPLETE path through every layer (schema, API, UI, tests)
- A completed slice is demoable or verifiable on its own
- Prefer many thin slices over few thick ones
- The first slice should be the simplest possible end-to-end path (the "hello world" tracer bullet)
- Later slices add breadth: edge cases, additional user stories, polish
</vertical-slice-rules>

<cross-discipline-rules>
When a PRD requires **both a Go engineer and a React engineer**, split the work into separate issues per discipline — do NOT combine them into one issue.

- Create one **Go issue** (backend: API endpoints, data models, business logic, config)
- Create one **React issue** (frontend: UI components, state management, API integration)
- The Go issue is almost always a blocker for the React issue, because the frontend depends on the API contract being in place. Mark the React issue as "Blocked by #<go-issue>".
- Each issue must include a `## Discipline` section so engineers can self-select work (see template below).
- If the Go and React work for a slice can be parallelised (e.g. the API contract is agreed upfront via a mock or OpenAPI spec), state that explicitly and mark the React issue as "Not blocked — depends on agreed API contract".
- Apply this split per vertical slice, not globally. A single PRD may produce Go+React pairs for some slices and single-discipline issues for others.
</cross-discipline-rules>

### 4. Quiz the user

Present the proposed breakdown as a numbered list. For each slice, show:

- **Title**: short descriptive name
- **Discipline**: `Go`, `React`, or `Go + React` (if split into two issues, list them as a pair)
- **Layers touched**: which integration layers this slice cuts through
- **Blocked by**: which other slices (if any) must complete first
- **User stories covered**: which user stories from the PRD this addresses

Ask the user:

- Does the granularity feel right? (too coarse / too fine)
- Are the dependency relationships correct?
- Should any slices be merged or split further?
- Is the ordering right for the first tracer bullet?
- Are there any slices missing?
- For `Go + React` pairs: should the React issue be blocked by the Go issue, or can they proceed in parallel against an agreed API contract?

Iterate until the user approves the breakdown.

### 5. Create the GitHub issues

For each approved slice, create a GitHub issue using `gh issue create`. Use the issue body template below.

Create issues in dependency order (blockers first) so you can reference real issue numbers in the "Blocked by" field.

<issue-template>
## Parent PRD

#<prd-issue-number>

## Discipline

<!-- One of: Go (backend) | React (frontend) -->
**<Go (backend) | React (frontend)>**

## What to build

A concise description of this vertical slice. Describe the end-to-end behavior, not layer-by-layer implementation. Reference specific sections of the parent PRD rather than duplicating content.

## Acceptance criteria

- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Criterion 3

## Blocked by

- Blocked by #<issue-number> (if any)

Or "None - can start immediately" if no blockers.

## User stories addressed

Reference by number from the parent PRD:

- User story 3
- User story 7
  </issue-template>

After creating all issues, print a summary table:

```
| # | Title | Discipline | Blocked by | Status |
|---|-------|-----------|-----------|--------|
| 42 | Basic widget creation — API | Go | None | Ready |
| 43 | Basic widget creation — UI | React | #42 | Blocked |
| 44 | Widget listing — API | Go | None | Ready |
| 45 | Widget listing — UI | React | #44 | Blocked |
```

Do NOT close or modify the parent PRD issue.
