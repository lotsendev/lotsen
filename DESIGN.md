# Dirigent — Brand & Design System

A living reference for colours, typography, spacing, and component patterns across the dashboard and landing page.

---

## Brand Identity

**Dirigent** is a lightweight Docker orchestration tool for solo devs and VPS deployments. The aesthetic balances developer pragmatism with visual refinement — clean, approachable, and quietly elegant.

### Mascot
A corgi sitting at a navy keyboard. The mascot's palette is the direct source of the two brand colours: navy (keyboard) and orange (corgi coat).

### Wordmark
- Typeface: **Fraunces** (serif)
- Letter-spacing: `-0.03em`
- Colour: Navy `#1e2d6e` in light mode

---

## Colour Palette

Colours are defined in **OKLCH** for perceptual consistency and declared as CSS custom properties in `dashboard/src/index.css`.

### Light Mode

| Role | Token | OKLCH | Hex |
|---|---|---|---|
| Background | `--background` | `oklch(0.98 0.006 252)` | `#f8faff` |
| Foreground | `--foreground` | `oklch(0.23 0.10 265)` | `#1a2152` |
| Primary (navy) | `--primary` | `oklch(0.26 0.13 265)` | `#1e2d6e` |
| Accent (orange) | `--accent` | — | `#f5923e` |
| Secondary | `--secondary` | `oklch(0.96 0.013 252)` | `#eef2fc` |
| Muted text | `--muted-foreground` | `oklch(0.51 0.05 245)` | `#5a6b8a` |
| Card | `--card` | white | — |
| Border | `--border` | `rgba(30, 45, 110, 0.09)` | — |
| Sky / highlight | — | — | `#d4e6f8` |
| Destructive | `--destructive` | rose | — |

### Dark Mode

| Role | Value |
|---|---|
| Background | `oklch(0.17 0.015 265)` — deep navy |
| Card | `oklch(0.22 0.015 265)` |
| Text | `oklch(0.97 0 0)` — near white |
| Accent | `oklch(0.78 0.15 52)` — bright orange |
| Border | `oklch(1 0 0 / 10%)` |

Default theme is **dark**.

### Status / Semantic Colours

| State | Background | Text |
|---|---|---|
| Healthy / success | `emerald-100` | `emerald-800` |
| Deploying / warning | `amber-100` / `rgba(251,191,36,0.12)` | `amber-900` |
| Info | `sky-100` | `sky-800` |
| Error / destructive | `rose-100` | `rose-800` |

Deploying state uses a blinking dot (`@keyframes blink`) with `#f59e0b`.

---

## Typography

Loaded from Google Fonts.

### Typefaces

| Role | Family | Weights |
|---|---|---|
| Display / headings | **Fraunces** (serif) | 300–900 variable, optical sizing |
| Body / UI | **DM Sans** (sans-serif) | 300–900 variable, optical sizing |
| Code / metadata | **JetBrains Mono** (monospace) | 400, 500 |

### Scale

| Usage | Size | Weight | Tracking | Line-height |
|---|---|---|---|---|
| Section heading | 26px | 700 | `-0.02em` | 1.25 |
| Sub-heading | 15px | 600 | `-0.01em` | — |
| Body | 15px | 400 | normal | 1.75 |
| UI label / button | 13–14px | 500–600 | normal | — |
| Table header | 11px | 500 | `0.05em` | — |
| Monospace code | 10–13px | 400–500 | — | — |

**Rules:**
- Use **Fraunces** for brand moments: logo, page titles, card titles, hero copy.
- Use **DM Sans** for everything interactive: buttons, labels, descriptions, nav.
- Use **JetBrains Mono** for technical content: image names, uptime, terminal output, table headers.

---

## Spacing & Sizing

### Border Radius

Base: `--radius: 0.625rem` (10px). Derived variants:

| Token | Value |
|---|---|
| `rounded-sm` | 6px |
| `rounded-md` | 8px |
| `rounded-lg` | 10px (base) |
| `rounded-xl` | 12–16px (cards) |

### Shadows

```
--shadow-2xs:  0 1px 3px 0px (5% black)
--shadow-xs:   0 1px 3px 0px (5% black)
--shadow-sm:   0 1px 3px, 0 1px 2px -1px (10% black)
--shadow-md:   0 1px 3px, 0 2px 4px -1px (10% black)
--shadow-lg:   0 1px 3px, 0 4px 6px -1px (10% black)
--shadow-xl:   0 1px 3px, 0 8px 10px -1px (10% black)
--shadow-2xl:  0 1px 3px 0px (25% black)
```

### Layout Dimensions

| Element | Value |
|---|---|
| Max content width (website) | 1100px |
| Horizontal page padding | 32px |
| Navbar height | 60px |
| Sidebar width | `clamp(16rem, 22vw, 20rem)` |
| Sidebar (collapsed) | 3.5rem |
| Section gap | 64px |
| Card padding | 24px (p-6) |
| Card gap | 24px (gap-6) |
| Mobile breakpoint | 768px |

---

## Components

### Buttons

Variants follow CVA in `dashboard/src/components/ui/button.tsx`.

| Variant | Style |
|---|---|
| `default` | Navy bg, white text, `shadow-xs`, hover 90% opacity |
| `destructive` | Red bg, white text |
| `outline` | Transparent bg, border, hover accent |
| `secondary` | Light blue bg |
| `ghost` | No bg, hover accent |
| `link` | Underline, no bg |

Sizes: `sm` (py-1.5), `default` (py-2), `lg` (py-2.5), `icon` (square).

Focus ring: navy, 3px, 50% opacity.

### Cards

```
bg: --card
border: 1px solid --border
border-radius: rounded-xl
padding: p-6
shadow: shadow-sm
```

Sub-components: `CardHeader`, `CardTitle` (Fraunces semibold), `CardDescription` (muted, text-sm), `CardContent`, `CardFooter`, `CardAction` (grid column 2, spans 2 rows).

### Badges

```
display: inline-flex
border-radius: rounded-md
padding: px-2 py-0.5
font-size: text-xs font-medium
```

Variants: `default`, `secondary`, `success`, `warning`, `info`, `destructive`, `outline`.

Icons: `size-3`, `gap-1`.

### Status Badges (inline, technical)

```
display: inline-flex
gap: 5px
font-size: 11px, monospace
padding: 2px 8px
border-radius: 4px
```

Dot: 5px circle. Blinks when `deploying`.

### Tables

- Header: 11px, 500, uppercase, `0.05em` tracking, monospace font
- Header background: Light surface variant
- Row padding: 10px vertical
- Row separator: Subtle border
- Technical fields (image, uptime): monospace

### Sidebar

- Background: `--card`
- Item text: 13px
- Item radius: 6px, padding 6px
- Active: Sky-dim background, navy text
- Icon-only state at 3.5rem width

---

## Animations

```css
@keyframes fadeUp {
  from { opacity: 0; transform: translateY(18px); }
  to   { opacity: 1; transform: translateY(0); }
}

@keyframes blink {
  0%, 100% { opacity: 1; }
  50%       { opacity: 0; }
}

@keyframes float {
  0%, 100% { transform: translateY(0); }
  50%       { transform: translateY(-8px); }
}
```

| Class | Duration | Easing |
|---|---|---|
| `.fade-up` | 0.65s | `cubic-bezier(0.16, 1, 0.3, 1)` |
| `.delay-1` – `.delay-5` | +0.05s to +0.45s | staggered entry |
| `.cursor-blink` | 1.1s step-end infinite | terminal cursor |
| `.mascot-float` | 4s ease-in-out infinite | mascot hero |

Hover/focus transitions: `0.15s–0.2s ease`.

Navbar gets `backdrop-filter: blur(16px)` on scroll.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Framework | React 19 + Vite 6 |
| Styling | Tailwind CSS v4 (`@tailwindcss/vite`) |
| Components | Radix UI + shadcn/ui |
| Variant API | Class Variance Authority (CVA) |
| Class merging | `tailwind-merge` |
| Icons | Lucide React |
| Animations | `tw-animate-css` |

---

## Design Principles

1. **Mascot-derived palette.** Every colour decision traces back to the corgi mascot. Don't introduce colours that break this origin.
2. **Serif for brand, sans for UI, mono for data.** Never swap these roles.
3. **Navy primary, orange accent.** Orange is reserved for CTAs and highlights — use it sparingly.
4. **Generous whitespace.** Prefer breathing room over density.
5. **OKLCH for colour.** Use OKLCH when adding new CSS custom properties; avoid raw hex in tokens.
6. **Tailwind in the dashboard, custom properties on the website.** Keep the styling approach consistent per context.
7. **Dark by default.** The dashboard defaults to dark mode; design dark-first, then verify light.
