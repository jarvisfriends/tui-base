# Layout & Rendering Guidelines

This document captures the rules we discovered the hard way. Every item has a
concrete "wrong way / right way" so future implementations can be copy-pasted
instead of reasoned out from first principles.

---

## 1. Never use `fmt.Sprintf` / `len()` for column widths

**Why it breaks:** `fmt.Sprintf("%-20s", text)` pads by *byte count*, not
*terminal cell count*. Unicode characters occupy different numbers of cells:

| Character | Codepoints | UTF-8 bytes | Terminal cells |
|---|---|---|---|
| `A` | 1 | 1 | 1 |
| `⚠` U+26A0 | 1 | 3 | 1–2 (terminal-dependent) |
| `✔` U+2714 | 1 | 3 | 1 |
| `❌` U+274C | 1 | 3 | **2** |
| `✔️` U+2714+FE0F | 2 | 6 | 1 (variation selector is zero-width) |

Using `len()` or `len([]rune())` for width in any of these cases produces
misaligned columns.

**Wrong:**
```go
row := fmt.Sprintf("%-25s %-5s %-5s", name, col1, col2)
```

**Right:**
```go
import "charm.land/lipgloss/v2"

nameCell := lipgloss.NewStyle().Background(bg).Width(25).MaxWidth(25).Render(truncate(name, 25))
col1Cell  := lipgloss.NewStyle().Background(bg).Width(5).MaxWidth(5).Render(col1)
col2Cell  := lipgloss.NewStyle().Background(bg).Width(5).MaxWidth(5).Render(col2)
row       := lipgloss.JoinHorizontal(lipgloss.Top, nameCell, col1Cell, col2Cell)
```

When you need to measure an existing rendered string (one that may already
contain ANSI escape codes), always use:
```go
w := lipgloss.Width(rendered)   // correct — strips ANSI, counts cells
w := len(rendered)              // wrong — counts bytes
w := len([]rune(rendered))      // wrong — counts Unicode codepoints
```

---

## 2. Always fill the page background

**Why it breaks:** When a line is shorter than the terminal width, or when a
`lipgloss.Style` only sets the foreground, the cells to the right of the content
fall through to the terminal's default background. With a dark theme this causes
an inconsistent color band at the end of each line.

**Wrong:**
```go
// Only sets foreground — background is terminal default on short lines
lines = append(lines, c.Styles.Error.Render(row))
```

**Right — option A (wrap every render in a full-bg style):**
```go
bg := c.Styles.TextOnBg.GetBackground()
rowStyle := c.Styles.Error.Background(bg).Width(m.width)
lines = append(lines, rowStyle.Render(row))
```

**Right — option B (place all content in a filled container):**
```go
bg := c.Styles.TextOnBg.GetBackground()
container := lipgloss.NewStyle().
    Background(bg).
    Foreground(c.Styles.TextOnBg.GetForeground()).
    Width(m.width)
return tea.NewView(container.Render(strings.Join(lines, "\n")))
```

Set `tea.View.BackgroundColor` to match so the compositor also fills any
uncovered area:
```go
v.BackgroundColor = c.Styles.TextOnBg.GetBackground()
v.ForegroundColor = c.Styles.TextOnBg.GetForeground()
```

> **SSH caveat:** `v.BackgroundColor` sends an `OSC 11` escape sequence. Many
> SSH servers and terminal multiplexers (tmux, screen, PuTTY, some remote
> terminals) strip or ignore OSC sequences entirely. This means the terminal
> default background (usually black) bleeds through for any unstyled cell.
> **Do not rely on `BackgroundColor` alone.** Every character position must have
> an explicit ANSI background code via lipgloss. The `Background(bg).Width(n)`
> + `Background(bg).Height(n)` pattern is the only fully SSH-safe approach.

---

## 3. Truncate before rendering, not after

Passing a string longer than the target width to `lipgloss.Style.Width(n)` does
**not** clip it — it sets the *minimum* width. The text overflows. Always
truncate first:

```go
// safe helper
func cell(text string, w int, style lipgloss.Style) string {
    if lipgloss.Width(text) > w {
        runes := []rune(text)
        // trim until display width fits
        for lipgloss.Width(string(runes)) > w-1 && len(runes) > 0 {
            runes = runes[:len(runes)-1]
        }
        text = string(runes) + "…"
    }
    return style.Width(w).MaxWidth(w).Render(text)
}
```

---

## 4. Use `lipgloss.JoinHorizontal` for multi-column rows

**Why:** `lipgloss.JoinHorizontal(lipgloss.Top, cols...)` correctly measures
each cell with `lipgloss.Width()` internally and produces a string whose total
display width is the sum of the column widths.

```go
// Build each cell with its own style+width, then join
cols := []string{
    nameStyle.Width(nameW).Render(name),
    statusStyle.Width(statusW).Render(status),
    sslStyle.Width(sslW).Render(ssl),
}
row := lipgloss.JoinHorizontal(lipgloss.Top, cols...)
```

---

## 5. Do not use `fmt.Sprintf("%-*s", ...)` with emoji / Unicode symbols

Status symbols used across the codebase:

| Symbol | Display width | Safe `Width()` value |
|---|---|---|
| `✔` U+2714 | 1 | 1 |
| `⚠` U+26A0 | 1 (narrow) | 2 (safe margin) |
| `❌` U+274C | **2** | 2 |
| `✅` U+2705 | **2** | 2 |
| `-` | 1 | 1 |

Set every DNS/status column to `Width(2)` to accommodate the widest symbol.

---

## 6. Width/Height for bordered boxes — lipgloss v2 outer semantics

**In lipgloss v2, `Width()` and `Height()` set the OUTER dimensions** (total
terminal cells including border, padding, and content). Content area = Width − 2
(for a 1-cell border on each side).

**Wrong (under-sized box — 2 cells short):**
```go
inner := width - 2
border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
    Width(inner).   // outer = inner = width-2 → box is 2 cells too narrow!
    Height(innerH)
```

**Right (outer dimensions = terminal cell allocation):**
```go
inner  := width  - 2  // pass to inner content renderers
innerH := height - 2  // pass to inner content renderers

border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
    Width(width).   // outer = width → content area = width-2 = inner ✓
    Height(height)  // outer = height → content rows = height-2 = innerH ✓
```

Content functions still receive `inner`/`innerH` as their own width/height budgets.

This was a recurring bug in dash's `RenderWidget`: passing `Width(inner)` caused
every widget box to be 2 cells narrower than its allocated grid column, leaving a
2-cell gap at the right edge of every row.

---

## 7. Always test rendered widths at multiple terminal sizes

Every page/component should have a test that:
1. Calls `Update(tea.WindowSizeMsg{Width: w, Height: h})`
2. Calls `View()` and splits the result by `"\n"`
3. Asserts `lipgloss.Width(line) <= w` for every line

See `common/layout_constraints_test.go` for the shared test helpers.

---

## 8. Background of status bar separator dots

The `bubbles/help` widget emits an ANSI *reset* (`\x1b[m`) before the separator
character `•`, stripping the status bar background from the gap. The status bar
calls `reapplyBg()` after `help.View()` to patch this. If a new component builds
its own key-hint line outside `BarModel`, it must do the same:

```go
left = reapplyBg(left, c.Styles.StatusBase.GetBackground())
```

`reapplyBg` is exported from the `status` package for exactly this purpose.

---

## 9. Color profile over SSH (washed-out / wrong colors)

**Symptom:** The app looks correct locally (and in WSL) but over SSH the theme
colors are washed-out, over-saturated, or simply *different* — e.g. a dark slate
background (`#1b2738`) renders as a bright, saturated blue.

**Root cause:** This is **not** a layout or background-fill bug — it is color
*quantization*. lipgloss emits 24-bit colors; Bubble Tea's renderer downsamples
them to whatever color profile it detected. Profile detection
(`charmbracelet/colorprofile`) decides:

| Signal | Result |
|---|---|
| `COLORTERM=truecolor` / `24bit` | **TrueColor** (24-bit, exact) |
| `TERM=xterm-256color`, no `COLORTERM` | **ANSI256** (8-bit, quantized) |

SSH does **not** forward `COLORTERM` unless the client's `SendEnv` *and* the
server's `AcceptEnv` are both configured. So a remote process running inside a
TrueColor terminal still sees only `TERM=xterm-256color` and conservatively
picks **ANSI256** — every theme color is then snapped to the nearest 256-color
palette entry, which can be wildly off for dark, low-saturation colors.

Use the inspector's **Terminal & Theme** section to confirm: compare
`Color Profile` (TrueColor vs ANSI256) between the two sessions. When it shows
`ANSI256` + `SSH: YES` + `COLORTERM: (not set)`, this is the cause.

**Fixes (in order of preference):**

1. **Environment (fixes every TUI on that host):** set `COLORTERM=truecolor` in
   the remote shell rc (`~/.bashrc`, `~/.zshrc`, `~/.profile`).
2. **Per-app override (no server changes):** run with
   `TUI_BASE_COLOR_PROFILE=truecolor`. tui-base honors this via
   `router.NewProgram` / `router.ForcedColorProfile`.
3. **SSH config (forward the var):** `SendEnv COLORTERM` on the client +
   `AcceptEnv COLORTERM` on the server.

**Framework rule:** every app built on tui-base MUST construct its program with
`router.NewProgram(model, opts...)` rather than `tea.NewProgram` directly, so the
`TUI_BASE_COLOR_PROFILE` override is honored consistently. Forcing TrueColor is
opt-in (never automatic) because a genuinely 256-color-only terminal would be
broken by forcing 24-bit output.

### 9a. Two shades of the "same" color on one line (OSC vs SGR)

**Symptom:** within a single frame, adjacent cells show two *slightly different*
shades of what should be one background — most visibly over SSH/ANSI256.

**Mechanism (confirmed in `colorprofile/writer.go`):** the downsampling writer
only rewrites **SGR** sequences (`ESC[…m`). It passes everything else through
untouched. Bubble Tea emits `tea.View.BackgroundColor` as **OSC** (`SetBackgroundColor`,
see `cursed_renderer.go`), which is therefore **never downsampled**. So:

- lipgloss `Background(c.Bg)` content → SGR → **quantized** to nearest ANSI256.
- `View.BackgroundColor` terminal fill + any unstyled join-padding (which falls
  to the terminal default) → OSC → **exact 24-bit**.

Same intended hex, two different on-screen colors, side by side. On TrueColor
both paths are identical so it's invisible locally.

**Fix (one place):** convert the View's background/foreground through the active
profile yourself so the OSC value matches the quantized SGR cells:

```go
prof := router.EffectiveColorProfile() // forced or detected — mirrors Bubble Tea
v.BackgroundColor = prof.Convert(c.Styles.TextOnBg.GetBackground())
v.ForegroundColor = prof.Convert(c.Styles.TextOnBg.GetForeground())
```

The router does this for the whole app in `router.View()`. Child pages' own
`v.BackgroundColor` are discarded by the router, so this single conversion makes
the entire frame one uniform shade on every profile. (TrueColor → `Convert` is a
no-op.)
