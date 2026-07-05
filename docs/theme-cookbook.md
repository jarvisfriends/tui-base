# Theme Cookbook

The theme system is four independent axes (ADR-012): **tint** (any bubbletint
palette) × **style preset** (huh structure) × **mode** (light/dark) ×
**accessibility** (CVD engine). Users switch all four live from Settings.
This page is the consumer recipes.

## Rule zero

Never hardcode a color. Every render reads from the shared `*theme.AppStyle`
(pages get it nil-safely via `page.Base.Colors()`); the router mutates that
pointer in place on theme changes, so your next render is already correct.

```go
func (p *myPage) View() tea.View {
    c := p.Colors()
    title := c.Styles.Title.Render("Orders")
    ...
}
```

## Semantic slots

| Slot | Use for |
|---|---|
| `c.Styles.Title` / `Subtitle` | headings / secondary text and section headers (`Subtitle.Bold(true)`) |
| `c.Styles.TextOnBg` | body text on the main background |
| `c.Styles.Item` / `SelectedItem` | list rows / the selected row |
| `c.SelectionBg` / `c.SelectionFg` | continuous selection bars (style every segment of the row, including separators) |
| `c.Accent` | focus and emphasis |
| `c.Styles.Success` / `Warning` / `Error` | status coloring |
| `c.Styles.Dim` | de-emphasized hints |

## Themed bubbles widgets (TC-1)

```go
tbl.SetStyles(theme.TableStyles(c))            // bubbles/table
delegate.Styles = theme.ListDelegateStyles(c)  // bubbles/list default delegate
sp.Style = theme.SpinnerStyle(c)               // bubbles/spinner
from, to := theme.ProgressGradient(c)          // bubbles/progress
bar := progress.New(progress.WithGradient(from, to))
```

Re-apply these in `View()` (they are cheap) so live theme switching restyles
your widgets without extra plumbing.

## huh forms

Pass `theme.HuhThemeFunc()` so forms follow the active tint/preset/mode:

```go
form := huh.NewForm(groups...).WithTheme(theme.HuhThemeFunc())
```

## Verifying you got it right

`testutil.CheckThemeResponsive` fails if switching themes doesn't change your
rendered colors — add it to your conformance test so hardcoded colors can't
creep back in.
