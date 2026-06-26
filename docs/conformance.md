# Conformance testing for tui-base apps

tui-base ships reusable conformance checks in the `testutil` package so every
derived app (dash, media, plex-maint, …) can be held to the same framework
invariants — the things that are easy to break when you hand-write pages. They
run as ordinary **unit tests**: they drive your `router.RouterModel` purely
through messages (resize, theme, page-switch, key) and assert on the rendered
frame. No running terminal or live backend required.

## Usage

```go
func TestMyAppConforms(t *testing.T) {
    build := func() *router.RouterModel {
        return router.NewWithOptions(router.Options{
            AppName: "My App", DefaultPage: "Home", ExtraPages: myPages(),
        })
    }

    t.Run("FitsViewport", func(t *testing.T) {
        testutil.CheckFitsViewport(t, build())
    })

    t.Run("ThemeResponsive", func(t *testing.T) {
        testutil.CheckThemeResponsive(t, build(),
            settings.ThemeMsg{ID: "dracula", Mode: "dark", ApplyPreferences: true},
            settings.ThemeMsg{ID: "dracula", Mode: "light", ApplyPreferences: true})
    })

    t.Run("StatusBarVisibleEverywhere", func(t *testing.T) {
        m := build()
        states := []tea.Msg{
            navigation.SelectedMsg{PageIndex: 0},
            navigation.SelectedMsg{PageIndex: 1},
            tea.KeyPressMsg{Text: "ctrl+d"}, // inspector overlay
            tea.KeyPressMsg{Text: "ctrl+,"}, // settings
        }
        testutil.CheckStatusBarVisible(t, m, states)
    })
}
```

(See `router/conformance_test.go` for the reference, and
`jarvisfriends/media/tui/conformance_test.go` for a derived-app example.)

## What each check guarantees

| Check | Catches |
|---|---|
| `CheckFitsViewport` / `AssertBounds` | Content larger than the screen that is **not clipped or scrolled** (frame taller than the terminal, or any line wider than it) at every standard size — the most common TUI corruption bug. Use `testutil.AssertBounds(t, model, w, h)` in your unit tests to prove a component properly paginates or truncates its content. |
| `CheckStatusBarVisible` | The **status bar disappearing** on some page, overlay, or prompt. Drive it with page switches + overlay/prompt toggles; it asserts the status bar's text is still in the frame whenever visible. Requires the model to implement `testutil.StatusProvider` (the router does, via `StatusBarContent()`). |
| `CheckThemeResponsive` | Pages using **hard-coded colors** instead of the shared theme — switching to a drastically different theme must change the rendered ANSI colors. |
| `CheckNoLineOverflow` / `CheckNoBorderOverflow` (legacy) | Lines / bordered boxes overflowing at narrow widths. Superseded by `testutil.AssertBounds`. |

## Notes
- These exercise messages **at different layers** (window-size, theme, navigation,
  key) — the same seam used to build richer scenarios.
- A runtime **"Conformance" Inspector tab** (live pass/fail against the running app)
  is planned as a complement for engineers who want to see results in-app — see
  `.github/ROADMAP.md`. The unit-test form above is preferred (CI-friendly).
