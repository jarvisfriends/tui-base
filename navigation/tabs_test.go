package navigation

import (
	"strings"
	"testing"

	"github.com/jarvisfriends/tui-base/theme"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"
)

func init() {
	tint.NewDefaultRegistry()
}

func TestNewTabs(t *testing.T) {
	tabs := NewTabs()
	if len(tabs.Pages) != 3 {
		t.Errorf("expected 3 default pages, got %d", len(tabs.Pages))
	}
	if tabs.ActiveIndex != 0 {
		t.Errorf("expected active index 0, got %d", tabs.ActiveIndex)
	}
	if tabs.HoverIndex != -1 {
		t.Errorf("expected hover index -1, got %d", tabs.HoverIndex)
	}

	if cmd := tabs.Init(); cmd != nil {
		t.Error("expected Init to return nil command")
	}
}

func TestTabsGettersSettersAndDock(t *testing.T) {
	tabs := NewTabs()

	// 1. Pages
	pages := []Page{{ID: "p1", Title: "P1"}, {ID: "p2", Title: "P2"}}
	tabs.SetPages(pages)
	if got := tabs.GetPages(); len(got) != 2 || got[0].ID != "p1" {
		t.Errorf("unexpected pages list: %+v", got)
	}

	// 2. ActiveIndex
	tabs.SetActiveIndex(1)
	if tabs.GetActiveIndex() != 1 {
		t.Errorf("expected active index 1, got %d", tabs.GetActiveIndex())
	}

	// 3. Width & Height
	if tabs.Width() != 0 {
		t.Errorf("expected Width 0, got %d", tabs.Width())
	}

	// Setup standard theme and size
	_ = theme.SetCurrentTint("dracula")
	tabs.SetColors(theme.Active())
	tabs.width = 80
	h := tabs.Height()
	if h <= 0 {
		t.Errorf("expected positive height, got %d", h)
	}

	// 4. Dock
	if tabs.Dock() != DockTop {
		t.Errorf("expected DockTop, got %v", tabs.Dock())
	}
}

func TestTabsUpdateWindowSizeAndHover(t *testing.T) {
	tabs := NewTabs()

	// Resizing
	m, cmd := tabs.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	updated := m.(*Tabs)
	if updated.width != 100 || updated.height != 40 {
		t.Errorf("Resize failed: width=%d height=%d", updated.width, updated.height)
	}
	if cmd != nil {
		t.Error("expected nil cmd on resize")
	}

	// Hover msg
	m, cmd = tabs.Update(TabHoverMsg{Index: 1})
	updated = m.(*Tabs)
	if updated.HoverIndex != 1 {
		t.Errorf("expected HoverIndex 1, got %d", updated.HoverIndex)
	}
	if cmd != nil {
		t.Error("expected nil cmd on hover")
	}
}

func TestTabsUpdateKeyPresses(t *testing.T) {
	tabs := NewTabs()
	tabs.Pages = []Page{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}, {ID: "c", Title: "C"}}
	tabs.ActiveIndex = 1

	// 1. Right key (move to B -> C)
	m, cmd := tabs.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	updated := m.(*Tabs)
	if updated.ActiveIndex != 2 {
		t.Errorf("expected index 2, got %d", updated.ActiveIndex)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd on key press")
	}
	msg := cmd()
	if sel, ok := msg.(SelectedMsg); !ok || sel.PageIndex != 2 {
		t.Errorf("unexpected SelectedMsg: %+v", msg)
	}

	// 2. Tab key (move to C -> A)
	m, _ = tabs.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	updated = m.(*Tabs)
	if updated.ActiveIndex != 0 {
		t.Errorf("expected index 0, got %d", updated.ActiveIndex)
	}

	// 3. Left key (move to A -> C)
	m, _ = tabs.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	updated = m.(*Tabs)
	if updated.ActiveIndex != 2 {
		t.Errorf("expected index 2, got %d", updated.ActiveIndex)
	}

	// 4. Shift+Tab (move to C -> B)
	// In code: case "left", "shift+tab": (checks keyMsg.String() which is "shift+tab")
	// Let's pass via KeyPressMsg with Text="shift+tab"
	m, _ = tabs.Update(tea.KeyPressMsg{Text: "shift+tab"})
	updated = m.(*Tabs)
	if updated.ActiveIndex != 1 {
		t.Errorf("expected index 1, got %d", updated.ActiveIndex)
	}

	// 5. Enter key
	m, cmd = tabs.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated = m.(*Tabs)
	if updated.ActiveIndex != 1 {
		t.Errorf("expected index 1, got %d", updated.ActiveIndex)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd on Enter")
	}
}

func TestComputeTabWindow(t *testing.T) {
	widths := []int{10, 10, 10, 10, 10} // 5 tabs, 50 cols total

	// All tabs fit: full range, no arrows.
	first, last, sl, sr := computeTabWindow(widths, 60, 2, 3, 3)
	if first != 0 || last != 4 || sl || sr {
		t.Errorf("fit-all: got first=%d last=%d showLeft=%v showRight=%v; want 0,4,false,false", first, last, sl, sr)
	}

	// Overflow with active at the far left: no left arrow, right arrow shown,
	// and the active tab is within the window.
	first, last, sl, sr = computeTabWindow(widths, 25, 0, 3, 3)
	if sl {
		t.Errorf("active-left: expected no left arrow")
	}
	if !sr {
		t.Errorf("active-left: expected right arrow (tabs clipped on the right)")
	}
	if first > 0 || last < 0 {
		t.Errorf("active-left: active 0 not visible (first=%d last=%d)", first, last)
	}

	// Overflow with active at the far right: left arrow shown, no right arrow,
	// and the last tab is the active one.
	first, last, sl, sr = computeTabWindow(widths, 25, 4, 3, 3)
	if !sl {
		t.Errorf("active-right: expected left arrow (tabs clipped on the left)")
	}
	if sr {
		t.Errorf("active-right: expected no right arrow")
	}
	if last != 4 || first > 4 {
		t.Errorf("active-right: active 4 not the last visible (first=%d last=%d)", first, last)
	}

	// Empty page set is handled gracefully.
	if f, l, _, _ := computeTabWindow(nil, 25, 0, 3, 3); f != 0 || l != -1 {
		t.Errorf("empty: got first=%d last=%d; want 0,-1", f, l)
	}
}

// TestTabsHorizontalScrollNoWrap verifies that with more tabs than fit, the row
// scrolls horizontally (stays a single tab-height tall) instead of wrapping onto
// extra lines, and that the active tab remains rendered.
func TestTabsHorizontalScrollNoWrap(t *testing.T) {
	tabs := NewTabs()
	tabs.Pages = []Page{
		{ID: "p1", Title: "Alpha"},
		{ID: "p2", Title: "Bravo"},
		{ID: "p3", Title: "Charlie"},
		{ID: "p4", Title: "Delta"},
		{ID: "p5", Title: "Echo"},
		{ID: "p6", Title: "Foxtrot"},
	}
	_ = theme.SetCurrentTint("dracula")
	tabs.SetColors(theme.Active())
	tabs.width = 24 // deliberately too narrow for all six tabs

	// Height of a single rendered tab (the row must never exceed this).
	singleTabHeight := lipgloss.Height(theme.Active().Styles.TabInactive.
		Border(lipgloss.RoundedBorder(), true).Padding(0, 1).Render("X"))

	for _, active := range []int{0, 3, 5} {
		tabs.ActiveIndex = active
		v := tabs.View()
		if got := lipgloss.Height(v.Content); got != singleTabHeight {
			t.Errorf("active=%d: row height %d; want %d (the row wrapped instead of scrolling)", active, got, singleTabHeight)
		}
		if !strings.Contains(v.Content, tabs.Pages[active].Title) {
			t.Errorf("active=%d: active tab %q not visible in scrolled row", active, tabs.Pages[active].Title)
		}
	}
}

func TestTabsMouseInteractions(t *testing.T) {
	tabs := NewTabs()
	tabs.Pages = []Page{
		{ID: "p1", Title: "Page One"},
		{ID: "p2", Title: "Page Two"},
	}
	_ = theme.SetCurrentTint("dracula")
	tabs.SetColors(theme.Active())
	tabs.width = 80

	v := tabs.View()
	if v.Content == "" {
		t.Fatal("expected non-empty tabs View content")
	}
	if v.OnMouse == nil {
		t.Fatal("expected OnMouse handler to be populated")
	}

	// Click tab index 1 (Page Two)
	// We need coordinates. Starts/ends are computed dynamically inside View().
	// Page One title is 8 chars, padding 0,1 -> total styled length ~12 chars.
	// So clicking at X=15, Y=0 should land on Page Two (tab 1).
	cmd := v.OnMouse(tea.MouseReleaseMsg(tea.Mouse{X: 15, Y: 0, Button: tea.MouseLeft}))
	if cmd == nil {
		t.Fatal("expected non-nil cmd on left click")
	}
	msg := cmd()
	if sel, ok := msg.(SelectedMsg); !ok || sel.PageIndex != 1 {
		t.Errorf("expected SelectedMsg for page 1, got %+v", msg)
	}
	if tabs.ActiveIndex != 1 {
		t.Errorf("expected active index 1, got %d", tabs.ActiveIndex)
	}

	// Click with non-left button should do nothing
	cmdNonLeft := v.OnMouse(tea.MouseReleaseMsg(tea.Mouse{X: 15, Y: 0, Button: tea.MouseRight}))
	if cmdNonLeft != nil {
		t.Error("expected nil cmd on right click")
	}

	// Click outside vertical bounds
	cmdOutY := v.OnMouse(tea.MouseReleaseMsg(tea.Mouse{X: 15, Y: 10, Button: tea.MouseLeft}))
	if cmdOutY != nil {
		t.Error("expected nil cmd when clicking outside vertical bounds")
	}

	// Mouse motion (hover) inside tab 0
	cmdHover := v.OnMouse(tea.MouseMotionMsg(tea.Mouse{X: 2, Y: 0}))
	if cmdHover == nil {
		t.Fatal("expected non-nil cmd on hover motion")
	}
	msgHover := cmdHover()
	if h, ok := msgHover.(TabHoverMsg); !ok || h.Index != 0 {
		t.Errorf("expected TabHoverMsg(0), got %+v", msgHover)
	}

	// Mouse motion outside vertical bounds (clear hover)
	tabs.HoverIndex = 0
	cmdHoverOut := v.OnMouse(tea.MouseMotionMsg(tea.Mouse{X: 2, Y: 10}))
	if cmdHoverOut == nil {
		t.Fatal("expected non-nil cmd to clear hover")
	}
	msgHoverOut := cmdHoverOut()
	if h, ok := msgHoverOut.(TabHoverMsg); !ok || h.Index != -1 {
		t.Errorf("expected TabHoverMsg(-1), got %+v", msgHoverOut)
	}
}
