// Package status renders the bottom status bar and its attached surfaces:
// key-binding help (short and expanded), the settings/notification/info icon
// cluster with click regions, the notification history panel, and the
// centered app-info modal.
//
// BarModel is the tea.Model the router embeds; it exposes precomputed
// ClickRegions so mouse hit-testing never parses ANSI output. The history
// panel and info modal are rendered here but composited and made modal by the
// router's overlay stack.
package status
