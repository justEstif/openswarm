# Site

The GitHub Pages landing page for openswarm lives at `docs/index.html`. Deployed automatically on push to `main` via a GitHub Actions Pages workflow. Single static file — no build step, no framework, no bundler.

## Visual Design

Surgical terminal-dark aesthetic: near-black canvas, one green accent (`#22c55e`), monospace brand identity. The visual language mirrors the tool itself — precise, minimal, no decoration that doesn't carry meaning.

### Color System

Two-layer palette: surface colors and semantic colors. All defined as CSS custom properties so the whole theme can be shifted by overriding `:root`.

- `--bg: #0b0b0b` — page background
- `--surface: #131313` — elevated surfaces (install pill, code blocks)
- `--border / --border-mid` — `#1e1e1e` / `#252525` — dividers and box edges
- `--fg / --fg-muted / --fg-faint` — `#e2e2e2` / `#6b6b6b` / `#3d3d3d` — text hierarchy
- `--accent: #22c55e` — single accent, used for: eyebrow labels, section tags, copy indicator, live badges, `$` prompt

No second accent. If a future feature needs a warning or error state, use the standard semantic colors (`--warn-bg`, `--bad-bg`) from missing.css.

### Typography

Two fonts only: `system-ui` (body) and `var(--mono-font)` (brand, labels, code). The brand is always monospace. Section tags use uppercase + wide letter-spacing for rhythm without a third typeface.

missing.css provides the base scale; custom overrides adjust only `--main-font` and `--mono-font`.

## Content Structure

Four sections, each with one job. The sequence follows the frontend skill's default landing page order.

### Hero

Converts — gets the visitor to install or click through to GitHub. Contains: product name (largest text on the page), one-sentence promise, install command (copy-on-click), and two action links. No feature list, no stats, no logo cloud.

The install command is the visual anchor of the hero. It doubles as the primary CTA: reading it *is* the instruction.

### Subsystems

Explains — answers "what does it do?" Six subsystem entries as a ruled list (`code label` + one-line description). Multiplexer support badges at the bottom. No cards, no icons, no nested hierarchy.

Each entry must fit in one sentence. If it can't, the description is too broad.

### Design

Proves — answers "why should I trust it?" Four numbered principles from the architecture, each with a short title and one sentence.

Ordered by most fundamental constraint first: file-backed → agent-friendly → multiplexer-agnostic → complexity hiding.

### Quickstart

Converts again — turns interest into a running instance. Three numbered steps: install, init, use. A `<details>` element holds alternative install methods (Homebrew, Go, mise) so they don't dilute the primary path.

## Interactions

Three intentional motions. Each has a clear job.

### Hero Entrance

Five elements stagger-animate on load: eyebrow → title → tagline → install pill → actions. Each delayed ~150ms from the previous.

Title uses `riseup` (translateY + opacity); others use `fadein` (opacity only). Creates reading order without effort.

### Scroll Reveal

Sections below the fold use `IntersectionObserver` with `threshold: 0.08`. Each section fades in and rises 18px when 8% of it enters the viewport. Observed once; unobserved after triggering to avoid re-animation on scroll-up.

### Copy Flash

The install pill is keyboard-accessible (`tabindex="0"`, Enter/Space triggers copy). Uses `navigator.clipboard` — fails silently on unsupported browsers.

On success: border turns accent, hint text changes to "copied!", both revert after 2.2s.

## Technical Choices

One file, no dependencies beyond the missing.css CDN link. Rationale: the page has no dynamic data, no routing, and no state beyond the copy-flash timeout. A build pipeline would add complexity with no benefit.

**missing.css** (`unpkg.com/missing.css@1.2.0`) provides the typographic base, CSS reset, and dark-theme variable scaffold. Custom CSS is ~200 lines of property overrides and layout. No missing.css component classes are used — layout is plain flexbox/grid. The CDN version is pinned to `1.2.0` to prevent surprise updates.

**No JS framework** — the two JS tasks (copy-to-clipboard, IntersectionObserver) are 20 lines total. Adding a framework would cost 40–100 KB for functionality that fits in a `<script>` tag.

**GitHub Actions deployment** — `.github/workflows/pages.yml` uploads `docs/` on every push to `main` that touches the folder. Uses OIDC (`id-token: write`) — no `GITHUB_TOKEN` secret needed. The Pages source must be set to "GitHub Actions" in repo Settings → Pages.

## Update Checklist

When changing the site, consider:

- [ ] Version number in the hero eyebrow matches `npm/package.json`
- [ ] Subsystem list matches actual `swarm --help` output
- [ ] Multiplexer badges match [[lat.md/backends#Backends#Backend Coverage]]
- [ ] Quickstart steps match actual CLI commands
- [ ] `lat check` passes after any wiki link changes
