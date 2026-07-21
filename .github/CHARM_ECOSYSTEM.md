# Charm Ecosystem — Library Rules & API Patterns

> For project-level context and the UI glossary, see [`copilot-instructions.md`](copilot-instructions.md).
> For tasks, see [`ROADMAP.md`](ROADMAP.md).

---

## Core Stack

| Package | Import | Role |
|---|---|---|
| Bubble Tea | `charm.land/bubbletea/v2` | Elm-architecture MVU framework |
| Lip Gloss | `charm.land/lipgloss/v2` | Layout and styling (CSS for the terminal) |
| Bubbles | `charm.land/bubbles/v2` | Reusable components: viewport, textinput, spinner, table, list, filepicker, help, key |
| Huh | `charm.land/huh/v2` | Declarative form builder |
| Bubbletint | `github.com/lrstanley/bubbletint/v2` | Theme/tint registry; wraps all active palette operations |

---

## Strict Rules

### 1. v2 Vanity Imports Only

```go
import "charm.land/bubbletea/v2"                // ✅ Good — v2 vanity path
import "github.com/charmbracelet/bubbletea"     // ❌ Bad — v1 / old path
```

The vanity prefix `charm.land/…/v2` is the fastest indicator of a correct v2 dependency.
Any new library added via `go get` must **not** pull in `github.com/charmbracelet/bubbletea`
(the old module) as a transitive dependency; check `go.mod` after every `go get`.

### 2. Declarative Views

`View()` returns `tea.View` — **not a string**. The struct drives terminal state declaratively:

```go
func (m *Model) View() tea.View {
    v := tea.NewView(m.renderedContent)
    v.AltScreen       = true
    v.MouseMode       = tea.MouseModeCellMotion
    v.BackgroundColor = m.colors.Bg
    v.ForegroundColor = m.colors.Fg
    v.WindowTitle     = "MyApp – PageName"
    return v
}
```

`BackgroundColor` and `ForegroundColor` control the full terminal canvas — set them
on the root router view and on each page view so the terminal never flashes the
default colors between renders.

### 3. Bubble Tea Handles Color Detection

Do **not** use `lipgloss.AdaptiveColor`. Instead catch `tea.BackgroundColorMsg` in
`Update()` and rebuild styles from the live dark/light state:

```go
case tea.BackgroundColorMsg:
    isDark := msg.IsDark()
    m.styles = buildStyles(isDark)
```

### 4. Functions for KeyMaps

```go
km := textinput.DefaultKeyMap()  // ✅ Good — function call returns a fresh copy
km := textinput.DefaultKeyMap    // ❌ Bad — copies a package-level variable (aliased)
```

### 5. Never Block Update

All I/O (network, DB, filesystem, logging) must run in a `tea.Cmd` and return a
message on completion. `Update()` must return in microseconds.

```go
func saveSettingsCmd(path string, data []byte) tea.Cmd {
    return func() tea.Msg {
        if err := os.WriteFile(path, data, 0o644); err != nil {
            return ErrMsg{err}
        }
        return SavedMsg{}
    }
}
```

### 6. No Global Styles

Styles live on the model, not at package level. Terminal capabilities (color depth,
dark/light mode) can change at runtime; global styles capture a stale snapshot.

---

## Shared Color Pointer Pattern (`*theme.AppColors`)

The router owns one `*theme.AppColors`. All child components receive this pointer
via `SetColors(c *theme.AppColors)`. When the theme changes (e.g., `settings.ThemeMsg`),
the router mutates the value in place:

```go
// router.Update — on settings.ThemeMsg
tint.SetTintID(msg.ID)
*m.colors = theme.Active()       // mutate in place — all children see the new palette
return m, m.handleResizeCmd()    // force a resize pass so children re-render immediately
```

Children that hold `*theme.AppColors` never need to be re-wired. Any component that
needs to render calls `resolveColors()`:

```go
func (m *Model) resolveColors() theme.AppColors {
    if m.colors != nil { return *m.colors }
    return theme.Active()         // safe fallback for tests that skip wiring
}
```

> **Pattern note for newcomers:** This is a deliberate deviation from pure-MVU
> immutability. We use it here for performance (no deep-copy on every theme change)
> and because `AppColors` is read-only after assignment. Treat the pointer as a
> "current theme" signal, not as shared mutable state.

---

## Shared Notification Manager Pointer (`*notifications.Manager`)

Follows the **exact same pattern** as `*theme.AppColors`. The router creates one
`*notifications.Manager`, passes it to every child that needs it (`SetNotifManager`),
and all children mutate/read through the same pointer.

```go
// router.New()
m.notifMgr = notifications.NewManager()
_ = m.notifMgr.Load(configDir)          // load persisted history
m.notifMgr.SetPersistPath(configDir)
m.status.SetNotifManager(m.notifMgr)

// Handling add / dismiss messages:
case notifications.AddMsg, notifications.DismissMsg, notifications.DismissAllMsg:
    cmd := m.notifMgr.Handle(msg)
    return m, tea.Batch(cmd, m.handleResizeCmd())
```

### Severity Colors

`notifications.ColorForSeverity(s Severity) string` returns a hex string safe for
`lipgloss.Color(...)`. Never use the color constants directly — always call through
this function so severity → color mapping stays in one place.

```go
// ✅ Correct
borderColor := lipgloss.Color(notifications.ColorForSeverity(toast.Severity))

// ❌ Wrong — hard-codes colors, breaks if we ever remap severities
borderColor := lipgloss.Color("#FF5757")
```

| Severity | Badge | Hex | TTL |
|---|---|---|---|
| `SeverityInfo` | `INFO` | `#4FC3F7` | 5 s |
| `SeverityWarning` | `WARN` | `#F9C513` | 10 s |
| `SeverityError` | `ERR ` | `#FF5757` | 15 s |

### `lipgloss.Color` Is a Function, Not a Type (v2 gotcha)

In Lip Gloss v2, `lipgloss.Color` is **`func(s string) color.Color`** — it is not a
type you can cast to. Use it like a constructor, always passing a hex string:

```go
// ✅ Correct — call it as a function
col := lipgloss.Color("#FF5757")

// ❌ Wrong — type assertion, does not compile in v2
var col lipgloss.Color = "#FF5757"
```

---

## Overlays via `lipgloss.NewCanvas` and `NewCompositor`

The router uses both compositing styles:

- `lipgloss.NewCanvas` for fixed-position overlays such as the toast and the
    notification history panel.
- `lipgloss.NewCompositor` for centered overlays such as the info modal and the
    settings edit form.

When the notification history panel is **not** open and there is at least one active
notification, the router composites a toast in the lower-right corner using a Canvas:

```go
func (m *RouterModel) View() tea.View {
    // ... build contentStr from nav + page + status ...

    activeToasts := m.notifMgr.Active()
    if len(activeToasts) > 0 && !m.status.IsHistoryVisible() {
        toast := activeToasts[0]
        borderColor := lipgloss.Color(notifications.ColorForSeverity(toast.Severity))
        toastStyle := lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(borderColor).
            Padding(0, 1).
            MaxWidth(44)
        msg := string([]rune(toast.Content)[:min(40, len([]rune(toast.Content)))])
        toastStr := toastStyle.Render(msg)
		toastW, toastH := lipgloss.Size(panelStr)
        toastX := max(0, m.width-toastW-1)
        toastY := max(0, m.height-toastH-statusH-1)

        canvas := lipgloss.NewCanvas(m.width, m.height)
        canvas.Compose(lipgloss.NewLayer(contentStr))
        canvas.Compose(lipgloss.NewLayer(toastStr).X(toastX).Y(toastY))
        contentStr = canvas.Render()
    }
    // ...
}
```

**Key points:**
- `lipgloss.NewCanvas(w, h)` + `canvas.Compose(layer)` + `canvas.Render()` is the
    v2 Canvas API. It is a good fit for fixed-position overlays that do not need
    layered z-order management.
- `lipgloss.NewCompositor(...)` is still used for centered overlays.
- The first `Compose` call with no `.X().Y()` positions the base layer at (0,0).
- Toast is hidden while the history panel is open (`!m.status.IsHistoryVisible()`).
- `expireMsg` is returned as a delayed `tea.Cmd` by `Manager.Add()` — the router
  handles it in `Update()` via `m.notifMgr.Handle(msg)`.

---

## Huh v2 — Known Behaviors and Constraints

### `Select` Never Collapses When Blurred

`huh.Select.View()` **always** renders its full options viewport regardless of
focus state. There is no "compact when blurred" mode. Only `huh.FilePicker` collapses
when blurred. `WithHideFunc` exists only at the `*huh.Group` level, not per-field.

**Consequence:** you cannot use huh alone for a "compact list until focused" UX.
Use a custom overview panel (see below) and open a single-field form in an overlay.

### `Form.Init()` Double-Focus Bug (Multi-Form Approach — Superseded)

When multiple forms are live simultaneously, calling `.Init()` on all of them at
startup causes **all** to show focused styles. The old fix used `ensureSingleActiveMsg`
to blur inactive forms after init. **This approach was replaced** by the
Overview + Overlay pattern (see next section) — only one form is ever alive at a time.

### `Form.WithWidth(n)` Must Be Called on Every Resize While Editing

If the active overlay form is not re-sized when the terminal changes size, the compositor
tries to paint a form wider than the viewport, producing a "one column at a time" visual
crawl. Fix — call `WithWidth` inside the `WindowSizeMsg` handler:

```go
case tea.WindowSizeMsg:
    m.width, m.height = wmsg.Width, wmsg.Height
    if m.editing && m.editForm != nil {
        m.editForm.WithWidth(max(30, min(m.width-14, 120)))
    }
```

### Width Formula for the Overlay Form

The border + padding box around the form consumes ~6–8 cells (1+1 border, 2+2 padding,
plus compositor doesn't add margin). Use `m.width - 14` as a comfortable gutter:

```go
formW := max(30, min(m.width-14, 120))
m.editForm = f.WithWidth(formW)
```

### `huh.StateAborted` vs `Esc`

`StateAborted` fires when huh's own abort binding is triggered (ctrl+c inside the form).
A separate `tea.KeyMsg` check for `"esc"` at the top of `updateEditing` is needed to
give the user a standard escape key:

```go
func (m *Model) updateEditing(msg tea.Msg) (tea.Model, tea.Cmd) {
    if km, ok := msg.(tea.KeyMsg); ok && km.String() == "esc" {
        return m, m.abortEdit()
    }
    // … forward to form …
    switch m.editForm.State {
    case huh.StateAborted:
        return m, m.abortEdit()
    }
}
```

---

## Overview List + Per-Field Overlay Pattern

> **This is the established settings UX.** Prefer this over multi-pane huh layouts.

A compact read-only list renders one row per setting (label + current value). When the
user selects a row (Enter, Space, or click), a single-field `huh.Form` is created and
composited as an overlay. Submitting or aborting returns to the overview.

### settingItem Shape

```go
type settingItem struct {
    title     string
    value     func() string    // current value string for the overview row
    buildForm func() *huh.Form // creates a fresh single-field form on demand
    leftTrunc bool             // true for paths: show tail with leading …
}
```

### startEdit

```go
func (m *Model) startEdit() tea.Cmd {
    f := m.items[m.cursor].buildForm()
    if f == nil { return nil }
    m.editForm = f.WithWidth(max(30, min(m.width-14, 120)))
    m.editing = true
    return m.editForm.Init()
}
```

### abortEdit (revert + close)

```go
func (m *Model) abortEdit() tea.Cmd {
    _ = m.LoadFromFile(defaultSettingsFile)
    m.buildItems()
    id := m.ColorThemeID
    m.editing = false
    m.editForm = nil
    return func() tea.Msg { return ThemeMsg{ID: id} }
}
```

### CapturesKeys gating

```go
func (m *Model) CapturesKeys() bool { return m.editing }
```

The router must check `CapturesKeys()` before acting on global shortcuts (Tab, Esc,
Ctrl+…). When `true`, all key messages pass directly to the settings model.

---

## Compositing API (Lip Gloss v2)

Overlays render on top of the current page using `lipgloss.NewLayer` /
`NewCompositor`:

```go
baseLayer  := lipgloss.NewLayer(baseContent).ID("base").X(0).Y(0).Z(0)
overLayer  := lipgloss.NewLayer(overlayContent).ID("overlay").X(left).Y(top).Z(10)
result     := lipgloss.NewCompositor(baseLayer, overLayer).Render()
```

- `.X()` / `.Y()` — top-left corner (0-based terminal cells)
- `.Z()` — draw order (higher Z paints on top)

### Established Helper

```go
func (m *Model) compositeOverlay(base, overlay string) string {
    x := max(0, (m.width-lipgloss.Width(overlay))/2)
    y := max(0, (m.height-lipgloss.Height(overlay))/2)
    return lipgloss.NewCompositor(
        lipgloss.NewLayer(base).ID("base").X(0).Y(0).Z(0),
        lipgloss.NewLayer(overlay).ID("overlay").X(x).Y(y).Z(10),
    ).Render()
}
```

### Caching Overlay Geometry for Hit-Testing (Click Outside to Close)

After computing overlay position in `View()`, cache the geometry on the model so
`OnMouse` can determine whether a click falls inside the overlay:

```go
// In View():
m.overlayX, m.overlayY = overlayX, overlayY
m.overlayW, m.overlayH = overlayW, overlayH

// In View().OnMouse:
v.OnMouse = func(mm tea.MouseMsg) tea.Cmd {
    click, ok := mm.(tea.MouseClickMsg)
    if !ok { return nil }
    if m.editing {
        inside := click.X >= m.overlayX && click.X < m.overlayX+m.overlayW &&
                  click.Y >= m.overlayY && click.Y < m.overlayY+m.overlayH
        if !inside {
            return m.abortEdit()
        }
        return nil
    }
    // … normal overview click handling …
}
```

### Active & Planned Overlays

| Overlay | Trigger | Status |
|---|---|---|
| Per-field settings edit | Enter / click row in settings overview | Done |
| Inspector / Debug View | Always present; routed via router | Done (page, needs Ctrl+D overlay) |
| Confirmation Dialogs | Destructive actions | Planned |
| Notification Toasts | Status bar bell / alerts | Planned |
| Command Palette | Ctrl+P | Planned |

---

### Mouse Routing (Manual Dispatch)

Bubble Tea only calls the **top-level** `tea.View.OnMouse`. Child views do not
receive mouse events automatically, so the router converts global coordinates to
child-relative coordinates and forwards the event to the active child view's
`OnMouse` handler.

Current mouse behavior in this app:

- The root view enables `tea.MouseModeCellMotion` so click, release, motion, and
  wheel events are available.
- The router handles overlays first:
  - notification history panel: outside release closes, inside wheel scrolls
  - info modal: outside release closes, inside wheel scrolls
- For normal content, the router routes by layout geometry to the cached child
  views for the current frame.
- A `debug.MouseHighlightMsg` is emitted for routed mouse clicks and releases so
  the inspector can show where the router thought the event landed.

Supported child mouse patterns:

- `navigation.Sidebar`: left click / release selects a page.
- `navigation.Tabs`: left click / release selects a page, mouse motion updates hover.
- `status.BarModel`: left release hits status-bar click regions.
- `pages/settings.Model`: left click on the overview opens an overlay; clicking
    outside the overlay closes it.

See `router/router.go` `View().OnMouse` for the full sidebar / tabs dispatch logic.

---

## Guardrails

1. **No Skate/Charm Cloud.** Local YAML + Postgres only. Air-gap capable.
2. **No Glamour** unless complex markdown rendering is required. Use `bubbles/viewport` with styling.
3. **Use Huh for forms.** Never hand-build input arrays from raw `textinput` components.
4. **Verify new dependencies** — after `go get`, confirm the new module uses `charm.land/…/v2` vanity imports (not `github.com/charmbracelet/…`). Avoid pulling in `charmbracelet/log`, `charmbracelet/x/cellbuf`, etc., unless pinned to a version known-compatible with the project's transitive graph.
5. **Logging** — use the project's `github.com/jarvisfriends/tui-base/logging` wrapper. It routes to a file (temp dir by default) and to the inspector via subscriber callbacks. Do not use `fmt.Printf` or `log.Println` inside UI code; route through `logging.Infof / Debugf / Warnf / Errorf`.
6. **Run `modernize -fix ./...` before committing.** The tool auto-applies Go idiom improvements (`min`/`max` builtins, `strings.Cut`, etc.). Install once: `go install golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest`.

---

## Key Size / Layout Gotchas

### `lipgloss.Width()` on Rendered Strings

`lipgloss.Width(rendered)` returns **total rendered width** (borders + padding + content).
Content width = `Width - style.GetHorizontalFrameSize()`.

### `lipgloss.Style.Width(n)` Sets Total Width

`Width(n)` sets the *total* rendered width — border and padding are subtracted from `n`
before the content area is computed. Do **not** pre-subtract; pass the full intended
outer width:

```go
// ✅ Correct — lipgloss subtracts border (2) and padding (4) internally
style.Border(lipgloss.RoundedBorder()).Padding(0, 2).Width(totalW).Render(text)

// ❌ Wrong — the style will shrink the content area by another border+padding
style.Border(...).Padding(...).Width(totalW - 2 - 4).Render(text)
```

### `handleResizeCmd()` After Theme Changes

After a theme change triggers child re-renders, call `handleResizeCmd()` in the router
to immediately forward correct child sizes. Without this, children may briefly render
with stale dimensions and produce a visible flicker (centered → left-aligned).

### Row Width Accounting in Overview Lists

When building a columnar overview row manually, account for **every** consumed cell:

```
innerW  = totalW - horizontalPadding          // e.g. Padding(1,2) → subtract 4
labelW  = min(desiredLabel, innerW / 2)
valueW  = innerW - labelW - 3                 // 2 for cursor prefix "▶ "/" " + 1 separator
```

The separator space between label and value columns is easy to forget and causes
lipgloss to silently clip the last cell, making `truncate()` add `…` on values that
should fit.

### Left-Truncation for File Paths

For paths and other strings where the tail is more informative than the head, use
right-anchored truncation with a leading `…`:

```go
// truncateLeft keeps the tail, prepending "…" if the string is too long.
func truncateLeft(s string, maxW int) string {
    r := []rune(s)
    if len(r) <= maxW { return s }
    if maxW <= 1 { return "…" }
    return "…" + string(r[len(r)-(maxW-1):])
}
```

Use `truncate` (right-side) for labels and option names; use `truncateLeft` for
filesystem paths so the filename / most-recent-segment stays visible.

---

## Models That Could Become Standalone Libraries

> These are noted for future extraction — keep their APIs clean and dependency-minimal.

| Model / Package | What it does | Extraction notes |
|---|---|---|
| `navigation.Sidebar` | Scrollable vertical nav with mouse support | Very close to `bubbles/list`; consider migrating to `bubbles/list` (already in stack) with a custom delegate for the nav-item styling. Sidebar key + mouse logic would thin considerably. |
| `navigation.Tabs` | Horizontal tab bar with mouse click routing | Similar to `bubbles/tabs` if/when it ships; watch charm releases. Currently custom — extraction viable as `bubbletabs`. |
| `theme/` + `AppColors` | Semantic color mapping over bubbletint | Thin adapter; not worth standalone extraction unless we decouple from bubbletint entirely. |
| `logging/` | File-backed runtime logger with subscriber/fan-out | Useful as a standalone `bubblelog` package. Could grow a ring-buffer, level histogram, and export interface. |
| `pages/inspector` (`Model`) | In-app message inspector with mouse highlight | Prime candidate for standalone `bubbleinspector` library. Dependency: only `bubbletea/v2` and `lipgloss/v2`. |
| `status/` | Themed status bar with click regions | Close to `bubbles/statusbar` if it existed; currently custom. Extraction viable. Remove hard-coded notification seed before extracting. |

---

## Existing Charm Libraries Worth Evaluating

| Charm library | Current custom code | Recommendation |
|---|---|---|
| `charm.land/bubbles/v2` — `list` | `navigation.Sidebar` item rendering | Migrate nav items to a `list.Model` with a custom delegate; removes ~200 lines of manual key/mouse handling. |
| `charm.land/bubbles/v2` — `viewport` | `pages/inspector` scroll area | Replace the `MaxHeight` clipping with a `viewport.Model`; gives free scrolling, search, and keyboard navigation. |
| `charm.land/bubbles/v2` — `filepicker` | `huh.FilePicker` in settings | `huh.FilePicker` wraps `bubbles/filepicker` — already in use; correct approach. |
| `charm.land/bubbles/v2` — `help` | `status_bar.go` key hint rendering | Already using `key.Binding` correctly; wire into a `help.Model` for automatic short/full toggle rendering. |
