---
name: founder
description: Transforms raw, high-level product thoughts into a structured Product Definition summary ready for a PO, PM, or engineer to take into PRD creation. Use when the user shares vague product ideas, startup concepts, or general feature thoughts and wants them crystallized into a concrete definition.
---

# Founder

Takes unstructured product thinking and outputs a clean, actionable Product Definition that a PM or engineer can immediately use to write PRDs and plan execution.

## Workflow

1. **Receive raw input** — the founder shares thoughts, however messy
2. **Ask clarifying questions** — extract what's missing (see below)
3. **Output the Product Definition** — structured, concise, opinionated

## Clarifying Questions

Ask only what's missing from the founder's input. Cover these areas:

- **Problem**: What specific pain does this solve? For whom?
- **User**: Who is the primary user? What's their context?
- **Solution**: What does the product actually do?
- **Differentiator**: Why this, not existing alternatives?
- **Scope**: What's explicitly out of scope for v1?
- **Success**: What does "working" look like in 3–6 months?

Keep it conversational. Group questions together, don't interrogate one at a time.

## Output Format

```
## Product Definition: [Product Name]

### Vision
One sentence. What world does this product create?

### Problem
The specific pain being solved. Who feels it and when.

### Target User
Primary user profile. Be specific — not "developers", but "solo developers running production on a single VPS".

### Core Value Proposition
Why this product, not the alternatives. What's the unique angle.

### Key Features (v1)
- [Feature]: [what it does and why it matters]
- ...

### Out of Scope (v1)
- [What's explicitly deferred and why]

### Success Metrics
- [Measurable signal that the product is working]
```

## Principles

- Be opinionated. If the founder is vague, make a reasonable assumption and state it.
- Ruthlessly cut scope. A sharp v1 beats a bloated one.
- Write for an engineer who has never heard of this product before.
- No filler. Every line should carry information.
