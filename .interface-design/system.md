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

### Deployment Detail Page Pattern
- Back navigation is a styled `Link` (not a `Button`) — `text-muted-foreground hover:text-foreground transition-colors` — navigation weight, not action weight.
- Service identity block (`rounded-xl border border-border/60 bg-card p-5`):
  - Left: display-font name + StatusBadge inline, image with Package icon below.
  - Right: Edit button (`variant="outline" size="sm" h-7 gap-1.5 px-2.5 text-xs`) at top, domain link + ID below.
  - Domain, when present, is an `<a href="https://...">` with `ExternalLink` icon that fades in on `group-hover` (`opacity-0 group-hover:opacity-60`).
  - ID uses `text-[11px] text-muted-foreground/40` — tertiary, only for debugging.
- Section labels: `text-[11px] font-medium uppercase tracking-wider text-muted-foreground/60`.
- Config list items: `rounded-md bg-background/70 px-2.5 py-1.5 font-mono text-xs text-muted-foreground`.
- Store snapshot: collapsible section using `useState`, ChevronDown rotates 180° when open. Content is `JSON.stringify(deployment, null, 2)` in a `bg-background/70 rounded-lg border border-border/40 p-4` pre block.
- Edit action opens a Dialog reusing `EditDeploymentForm` with `hideHeader` + `className="mb-0 border-0 shadow-none"`.

### Dialog Pattern
- Dialogs use softened overlays and card-like content surfaces:
  - overlay: `bg-foreground/35` + slight blur,
  - content: `rounded-xl border-border/60 bg-card` with compact header spacing.
- Titles use `DM Sans` display mapping for consistency with page-level headings.
- Destructive dialogs include a dedicated warning inset before the confirmation field/action.
