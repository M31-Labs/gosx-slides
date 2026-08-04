# GopherCon 2026 deck visual system

## Visual System

Territory: **Dark Elegance**. The presentation sits on the M31 live Scene3D
starfield. Content feels like illuminated instrumentation: crisp structure,
generous negative space, and one warm signal color. The starfield is an
intentional platform proof, but it never competes with copy. A bounded
near-black glass plate protects each content region while unused canvas remains
fully live.

Typography:

- Display: `var(--font-display)` — Space Grotesk, weight 600–800.
- Body: `var(--font-body)` — Plus Jakarta Sans, weights 400 and 700.
- Mono: `var(--font-mono)` — JetBrains Mono, weight 500–700.
- Scale: the Aurora Perfect Fourth scale, `var(--fs-1)` through
  `var(--fs-6)` plus `var(--fs-body)`.

Color architecture:

- Dominant (60%): `var(--bg)`, the near-black blue canvas.
- Secondary (30%): `var(--surface)` and `var(--line)` for quiet technical
  panels and separators.
- Accent (10%): `var(--accent)`, the warm amber signal color.
- Text: `var(--fg)` for primary copy and `var(--fg-muted)` for supporting
  copy. The Aurora theme supplies AA-or-better contrast for both on the canvas.
- Content backing: `rgba(1, 1, 6, 0.78)` with blur, reduced saturation, reduced
  brightness, and a subtle token-based border. It ends above the caption band.
- The closing QR card alone uses `var(--qr-bg)` and `var(--qr-fg)` to preserve
  scanner contrast.

Motion:

- Philosophy: **Subtle**. The persistent Scene3D starfield moves slowly in six
  independent depth bands behind the bounded content plate, capped at 30 fps.
  The finale replaces it with a cropped galaxy surface and a deliberately
  exaggerated spectral cycle. Foreground movement is limited to slide fades
  and the small, audience-triggered syntax-tree disclosure.
- Fast: 200ms; medium: 400ms; slow: 700ms.
- Ease-out: `cubic-bezier(0.25, 1, 0.5, 1)`.
- Spring: `cubic-bezier(0.34, 1.56, 0.64, 1)`.
- Reduced-motion mode removes both live Scene3D engines and interactive
  transition movement, then uses the static local star texture.

Spacing:

- Base unit: 8px.
- Use the Aurora tokens `var(--sp-1)` through `var(--sp-5)`.
- Micro-spacing below `0.5rem` is reserved for borders, labels, and optical
  alignment.

Implementation rules:

- New deck CSS uses the theme tokens above rather than introducing literal
  colors, fonts, spacing, or type sizes.
- Dense reference slides use ruled panels, not generic floating card grids.
- Headings and the amber signal remain the strongest visual elements on every
  slide.
- The 20% caption-safe lower band remains free of essential content.
