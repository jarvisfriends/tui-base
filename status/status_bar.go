package status

import (
	"github.com/jarvisfriends/tui-base/keys"
	"github.com/jarvisfriends/tui-base/notifications"
	"github.com/jarvisfriends/tui-base/page"
	"github.com/jarvisfriends/tui-base/theme"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Bar is the bar at the bottom of our App that displays helpful info like key combos
// available for the current view. This implementation wraps the more featureful
// statusbar model (animation, overlay, click regions) and exposes the same
// tea.Model surface used elsewhere in the app.
type BarModel struct {
	page.Base
	help     *help.Model
	helpView tea.View
	keys     *keys.AppKeyMap

	// pageBindings, when non-nil, overrides the global keys display so each
	// page can show its own relevant shortcuts in the status bar.
	pageBindings help.KeyMap
	isVisible    bool

	sb          *UserNotificationOverlay
	lastRegions []ClickRegion // regions from the last SetWidth render

	// summaryProvider, when set, supplies the right-aligned status bar text
	// (e.g. the inspector's compact runtime summary shown when the inspector is
	// closed). It is evaluated on every render so the text stays current without
	// manual refresh calls.
	summaryProvider func() string
}

// SetSummaryProvider sets a callback that supplies the right-aligned status bar
// text, evaluated on every render. Pass nil to clear it.
func (b *BarModel) SetSummaryProvider(fn func() string) {
	b.summaryProvider = fn
	b.SetWidth(b.help.Width())
}

// summary returns the current right-aligned summary text, or "" when no
// provider is set.
func (b *BarModel) summary() string {
	if b.summaryProvider == nil {
		return ""
	}
	return b.summaryProvider()
}

// Regions returns the interactive click regions computed during the last
// SetWidth call. The router uses these for hit-testing without re-parsing
// ANSI-encoded output.
func (b *BarModel) Regions() []ClickRegion { return b.lastRegions }

// applyHelpStyles updates the help widget's internal styles to match the
// current theme palette. SetColors/Colors are inherited from page.Base.
func (b *BarModel) applyHelpStyles() {
	b.help.Styles = b.Colors().Styles.Help
}

func New() *BarModel {
	h := help.New()
	h.Styles = theme.Active().Styles.Help

	return &BarModel{
		help:      &h,
		helpView:  tea.NewView(""),
		sb:        NewUserNotificationOverlay(),
		isVisible: true,
	}
}

// ClickRegionMsg is emitted when the user clicks a named interactive region
// on the status bar (for example, the settings icon). The router will handle
// this message and perform navigation.
type ClickRegionMsg struct{ Name string }

func (b *BarModel) SetKeys(keys *keys.AppKeyMap) {
	b.keys = keys
	b.SetWidth(b.help.Width())
}

// SetPageBindings supplies page-specific key bindings that are shown in the
// status bar instead of the global router shortcuts. Pass nil to revert to
// the global key map.
func (b *BarModel) SetPageBindings(bindings help.KeyMap) {
	b.pageBindings = bindings
	b.SetWidth(b.help.Width())
}

// Init implements [tea.Model].
func (b *BarModel) Init() tea.Cmd { return nil }

// Update implements [tea.Model].
func (b *BarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Forward control messages to the internal statusbar model to drive
	// animations and TTLs. The statusbar.Handle method returns a tea.Cmd
	// when it wants the runtime to schedule ticks or timeouts.
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		b.SetWidth(m.Width)
		return b, nil
		// case tea.KeyMsg:
		// 	// switch keyMsg := msg.(type) {
		// 	// case tea.KeyPressMsg:
		// 	switch {
		// 	case key.Matches(m, b.keys.ToggleFullHelp):
		// 		b.help.ShowAll = !b.help.ShowAll
		// 		return b, nil
		// 	case key.Matches(m, b.keys.ToggleHelp):
		// 		return b, b.sb.ToggleVisibility()
		// 	}
		// 	// }
	}
	// Allow the internal statusbar to handle other control messages like
	// TickMsg, ToggleVisibilityMsg, AddMessageMsg, etc.
	return b, b.sb.Update(msg)
}

// View implements [tea.Model]. It composes overlay (if any) above the status
// line and exposes an OnMouse handler that maps clicks to ClickRegionMsg.
func (b *BarModel) View() tea.View {
	if !b.IsVisible() {
		return tea.NewView("")
	}
	return b.helpView
}

func (b *BarModel) SetWidth(width int) {
	// Apply theme colors to the help widget before rendering.
	b.applyHelpStyles()
	// ensure help width is up to date for the left content
	b.help.SetWidth(width)
	c := b.Colors()

	// When the active page provides its own key bindings show those; otherwise
	// fall back to the global router shortcuts.
	var left string
	if b.pageBindings != nil {
		left = b.help.View(b.pageBindings)
	} else {
		left = b.help.View(b.keys)
	}
	// The bubbles help widget emits \x1b[m (reset) before each inter-element
	// space, stripping the background. Re-apply StatusBg after every reset so
	// those spaces stay on the status bar background, not the terminal default.
	left = theme.ReapplyBg(left, c.Styles.StatusBase.GetBackground())

	// Render the status line using the internal statusbar renderer. The right
	// segment carries the optional summary (e.g. the inspector's runtime stats).
	statusLine, regions := b.sb.Render(width, left, b.summary())

	// Store regions so the router can access them without re-parsing ANSI output.
	b.lastRegions = regions

	// Icons live on the last row (row 0 in short-help, last row in full-help).
	statusLineRow := max(lipgloss.Height(statusLine)-1, 0)

	// Update the tea.View content and attach an OnMouse handler to map
	// clicks on interactive regions back into ClickRegionMsg messages.
	b.helpView.SetContent(c.Styles.StatusBase.Width(width).Render(statusLine))
	b.helpView.BackgroundColor = c.Styles.StatusBase.GetBackground()
	b.helpView.OnMouse = func(mm tea.MouseMsg) tea.Cmd {
		return func() tea.Msg {
			switch ev := mm.(type) {
			case tea.MouseReleaseMsg:
				me := ev.Mouse()
				// Only respond to clicks on the status bar row itself.
				// lipgloss.Height("") == 1, so never derive the row from an
				// empty overlay string — use the pre-computed row index instead.
				if me.Y != statusLineRow {
					return nil
				}
				// Use lipgloss.Width for ANSI-safe region hit-testing.
				x := me.X
				for _, r := range regions {
					if x >= r.Start && x <= r.End {
						return ClickRegionMsg{Name: r.Name}
					}
				}
			}
			return nil
		}
	}
}

// Height returns the height of the status bar view (always 1 when visible).
func (b *BarModel) Height() int {
	if b.sb.Visible() {
		return lipgloss.Height(b.helpView.Content)
	}
	return 0
}

func (b *BarModel) ToggleFullHelpVisible() {
	b.help.ShowAll = !b.help.ShowAll
	b.SetWidth(b.help.Width())
}

func (b *BarModel) IsVisible() bool { return b.isVisible }

func (b *BarModel) ToggleVisible() tea.Cmd {
	b.isVisible = !b.isVisible
	// if cmd := b.sb.ToggleVisibility(); cmd != nil {
	// 	return cmd
	// }
	// animation was in-flight; force immediate toggle for deterministic tests
	b.sb.ForceToggleVisibility()
	return nil
}

// ToggleNotifications toggles the notification history panel.
func (b *BarModel) ToggleNotifications() tea.Cmd { return b.sb.ToggleHistory() }

// IsHistoryVisible reports whether the history panel is currently open.
func (b *BarModel) IsHistoryVisible() bool { return b.sb.ShowHistory() }

// NotifHistoryCursorUp moves the history cursor up.
func (b *BarModel) NotifHistoryCursorUp() { b.sb.HistoryCursorUp() }

// NotifHistoryCursorDown moves the history cursor down.
func (b *BarModel) NotifHistoryCursorDown(max int) { b.sb.HistoryCursorDown(max) }

// HistoryCursor returns the current notification history cursor index.
func (b *BarModel) HistoryCursor() int { return b.sb.HistoryCursor() }

// CloseHistory closes the notification history panel.
func (b *BarModel) CloseHistory() { b.sb.CloseHistory() }

// SetNotifManager wires the shared notification manager to the status bar.
func (b *BarModel) SetNotifManager(nm *notifications.Manager) { b.sb.SetNotifManager(nm) }

// RenderHistoryOverlay returns the rendered notification history panel string
// (or "") so the router can composite it as a canvas layer. screenW and screenH
// are the full terminal dimensions used to cap the panel size.
func (b *BarModel) RenderHistoryOverlay(screenW, screenH int) string {
	return b.sb.RenderHistoryOverlay(screenW, screenH)
}

var _ tea.Model = (*BarModel)(nil)
