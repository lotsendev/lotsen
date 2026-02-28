# Dirigent Interface System

## Direction and Feel
- **Direction:** Fleet control board for deployment operations.
- **Human:** Solo operator managing services on a VPS, often triaging incidents quickly.
- **Feel:** Calm command center, balanced density, operational clarity over decorative UI.
- **Signature:** Manifest-style deployment rows (service identity, runtime payload, command rail).
- **Typography direction:** Unified sans-serif heading voice so page titles and card titles stay operational and neutral.

## Depth Strategy
- **Primary strategy:** Borders + subtle surface color shifts.
- **Surface behavior:** Quiet contrast jumps (`bg-card`, `bg-background/70`, `border-border/60`) with minimal shadow reliance.
- **State emphasis:** Semantic status color is reserved for health meaning (healthy/deploying/failed), not general decoration.

## Spacing System
- **Base unit:** 4px grid.
- **Common rhythm:** 8/12/16/20px increments for component interiors and section stacking.
- **Card/list cadence:** Deployment list uses `space-y-3` between manifest rows; row internals use grouped `space-y-3` with compact tag clusters.

## Key Component Patterns

### Deployment List Header
- Fleet summary strip with five compact counters: total, healthy, deploying, failed, idle.
- Control lane combines search and status filters in one scan line.
- Primary action (`Create deployment`) remains right-aligned and always visible.

### Deployment Manifest Row
- Each deployment is an `article` surface (not table row) with three zones:
  - **Identity:** Name link + status badge.
  - **Payload:** Image, route/domain, and compact runtime tags (ports, volumes, env vars).
  - **Command rail:** Details/Investigate, Edit, Delete.
- Failed services should remain visually and structurally triage-friendly.

### Deployment States
- **Loading:** Skeleton manifest rows (shape-aligned placeholders).
- **Error:** Retryable panel with clear runtime wording.
- **Empty (true):** First-deployment onboarding card.
- **Empty (filtered):** Separate “no matches” state with explicit clear filters action.

### Status Treatment
- Deployment status badges are title-cased labels with consistent minimum width for fast vertical scanning.
- Preserve semantic mapping:
  - idle -> secondary
  - deploying -> info
  - healthy -> success
  - failed -> destructive

### Create Deployment Dialog
- Dialog body follows staged setup flow:
  - **Signal strip:** `Image + runtime`, `Route + exposure`, `Ready for deploy`.
  - **Core identity:** Name and image as required fields in a shared section.
  - **Ingress:** Optional domain separated from required identity.
  - **Access control:** Basic auth as an explicit optional switch with contextual help.
- Dynamic sections (`env`, `ports`, `volumes`, `basic auth users`) use bordered inset surfaces with:
  - title + count badge,
  - secondary add action,
  - per-row remove action,
  - explicit empty message (`No entries yet`).
- Footer action stays right-aligned with a single primary submit action in create flow.

### Observability Page Frame
- System status, traffic, and settings pages use the same top-level framing pattern as deployments:
  - page title + description in the layout header,
  - then stacked `rounded-xl border border-border/60 bg-card` sections,
  - with inset `bg-background/70` sub-surfaces for dense telemetry blocks.

### Traffic Page Pattern
- Traffic screen is split into two sections:
  - **Traffic watch:** summary strip (total/suspicious/blocked/active blocked IPs) + blocked IP roster.
  - **Access ledger:** command-first filter rail (method chips + field filters + apply/clear actions) and bordered log table.
- Avoid native select controls for method filtering; use button chips with active/inactive states.
- Status and outcome in log rows use badges for fast scan-level parsing.

### System Status Pattern
- Service health uses manifest-style cards (API, orchestrator, docker, load balancer) with:
  - title + icon capsule,
  - explicit state rail,
  - checks block,
  - timestamp/freshness footer.
- Keep load balancer traffic telemetry nested inside its card as an inset sub-surface.
- Host metrics (CPU, RAM) use paired cards with monospace headline values and compact pressure badges.

### Settings Pattern
- Settings starts with a release runway section:
  - installed/latest/published/cache stat tiles,
  - upgrade action at section header level,
  - release notes inside an inset bordered container.
- Upgrade feedback states remain inline and semantic:
  - success strip with reload action,
  - failure strip with log excerpt preserved in monospace.

### Dialog Pattern
- Dialogs use softened overlays and card-like content surfaces:
  - overlay: `bg-foreground/35` + slight blur,
  - content: `rounded-xl border-border/60 bg-card` with compact header spacing.
- Titles use `DM Sans` display mapping for consistency with page-level headings.
- Destructive dialogs include a dedicated warning inset before the confirmation field/action.
