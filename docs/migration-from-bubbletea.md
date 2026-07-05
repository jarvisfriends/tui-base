# Migrating a plain Bubble Tea app to tui-base

You keep your models. tui-base replaces the plumbing you wrote around them:
routing, theming, status/help, notifications, and debug tooling.

## The mapping

| You had | tui-base gives you |
|---|---|
| One god-model switching between "screens" in `Update` | One `tea.Model` per page; the router owns switching (Tab/Shift+Tab, sidebar/tabs/topnav, mouse) |
| Hand-rolled help line | Status bar renders `help.KeyMap` from your page (`SetPageBindings`) with icons and click regions |
| Global lipgloss style vars | Shared `*theme.AppStyle`, four user-controlled axes, live switching |
| `fmt.Println` debugging (corrupting the screen) | `logging.*` + the Ctrl+D inspector's message log |
| Custom program bootstrap | `tuibase.Run(Options{...})` or `router.NewProgramWithContext` for SIGTERM-clean services |

## Steps

1. **Split screens into pages.** Each screen becomes a `tea.Model` embedding
   `page.Base`. In `Update`, handle `tea.WindowSizeMsg` via `SetSize`; in
   `View`, size your content with `Width()`/`Height()` and colors from
   `Colors()`.
2. **Register pages.**

   ```go
   err := tuibase.Run(tuibase.Options{
       AppName: "My App",
       ExtraPages: []tuibase.RegisteredPage{
           {Title: "Orders", Model: orders.New()},
           {Title: "Fleet",  Model: fleet.New()},
       },
   })
   ```

3. **Delete your color constants** and read theme slots instead
   (docs/theme-cookbook.md). Wrap huh forms with `theme.HuhThemeFunc()`.
4. **Move I/O into `tea.Cmd`s** if it isn't already — `Update` must never
   block (ADR-004). High-frequency background messages should implement
   `router.TargetedMsg` so they wake only their page.
5. **Adopt the conformance tests** so regressions stay caught:

   ```go
   func TestConforms(t *testing.T) {
       m := router.NewWithOptions(opts)
       testutil.CheckFitsViewport(t, m, states...)
       testutil.CheckStatusBarVisible(t, m, states)
       testutil.CheckThemeResponsive(t, m, themeA, themeB)
   }
   ```

The `examples/` directory has runnable versions of each stage: `minimal`
(nothing but options), `dashboard` (one custom page + themed widgets),
`multipage` (several pages + custom settings + graceful shutdown).

## Developing against a local checkout

Use a Go workspace instead of `replace` directives:

```bash
cd ~/code/myapp
go work init . ../tui-base
```

`go.work` is developer-local (do not commit it); your `go.mod` keeps the
normal tagged requirement while builds resolve tui-base from the sibling
checkout.
