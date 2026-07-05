package inspector

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/tui-base/theme"
)

// TestTabKeysCycleInspectorTabs verifies the inspector honors the
// application's default next/previous page keys (tab / shift+tab) for tab
// switching, in addition to the inspector-local ←/→ shortcuts.
func TestTabKeysCycleInspectorTabs(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	if m.activeTab != debugTabRuntime {
		t.Fatalf("initial tab = %v; want Runtime", m.activeTab)
	}

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.activeTab != debugTabInput {
		t.Fatalf("after tab: activeTab = %v; want Input", m.activeTab)
	}

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.activeTab != debugTabRuntime {
		t.Fatalf("after shift+tab: activeTab = %v; want Runtime", m.activeTab)
	}

	// Wrap-around backwards from the first tab.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.activeTab != debugTabSettings {
		t.Fatalf("shift+tab from first tab = %v; want Settings (wrap)", m.activeTab)
	}
}

// TestSetNavKeysRebindsTabSwitching verifies custom app navigation bindings
// (e.g. a user rebind applied via the router) drive inspector tab cycling.
func TestSetNavKeysRebindsTabSwitching(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m.SetNavKeys(
		key.NewBinding(key.WithKeys("ctrl+n"), key.WithHelp("ctrl+n", "next page")),
		key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "prev page")),
	)

	_, _ = m.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	if m.activeTab != debugTabInput {
		t.Fatalf("after ctrl+n: activeTab = %v; want Input", m.activeTab)
	}
	_, _ = m.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	if m.activeTab != debugTabRuntime {
		t.Fatalf("after ctrl+p: activeTab = %v; want Runtime", m.activeTab)
	}

	// The old defaults must no longer switch tabs once rebound.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.activeTab != debugTabRuntime {
		t.Fatalf("tab still switches tabs after rebinding to ctrl+n/ctrl+p")
	}
}

// TestTabKeysEscapeAccessibilityTab ensures the nav keys switch tabs even
// while the accessibility panel (which claims most keys) is active.
func TestTabKeysEscapeAccessibilityTab(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m.switchTab(debugTabAccessibility)

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.activeTab != debugTabLog {
		t.Fatalf("after tab on accessibility tab: activeTab = %v; want Log", m.activeTab)
	}
}

// TestTableSelectionUsesSemanticColors asserts data tables highlight the
// cursor row with the theme's SelectionBg/SelectionFg — the same convention as
// the sidebar and settings lists (the old style was invisible: text colors on
// the text background).
func TestTableSelectionUsesSemanticColors(t *testing.T) {
	t.Parallel()

	m := New()
	c := theme.Active()
	s := m.baseTableStyles(c)
	if got := s.Selected.GetBackground(); got != c.SelectionBg {
		t.Errorf("table Selected background = %v; want theme SelectionBg %v", got, c.SelectionBg)
	}
	if got := s.Selected.GetForeground(); got != c.SelectionFg {
		t.Errorf("table Selected foreground = %v; want theme SelectionFg %v", got, c.SelectionFg)
	}
}

// TestTerminalTabHasCategorySections asserts the Terminal & Theme tab renders
// titled category sections instead of the old run-together key/value rows.
func TestTerminalTabHasCategorySections(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	out := m.buildTermSection(theme.Active(), 116)

	for _, header := range []string{
		"Terminal Environment",
		"Colors & Profile",
		"Active Theme",
		"Process & Launch",
	} {
		if !strings.Contains(out, header) {
			t.Errorf("terminal tab is missing the %q section header", header)
		}
	}
}

// TestHorizontalWheelSwitchesInspectorTabs verifies tilt-wheel (and the
// shift+wheel terminal encoding) cycles the inspector tabs like ←/→, and that
// a plain vertical wheel on a table tab moves the row cursor instead.
func TestHorizontalWheelSwitchesInspectorTabs(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})

	wheel := func(btn tea.MouseButton, mod tea.KeyMod) {
		v := m.View()
		if v.OnMouse == nil {
			t.Fatal("inspector view has no OnMouse handler")
		}
		_ = v.OnMouse(tea.MouseWheelMsg(tea.Mouse{X: 5, Y: 5, Button: btn, Mod: mod}))
	}

	wheel(tea.MouseWheelRight, 0)
	if m.activeTab != debugTabInput {
		t.Fatalf("wheel-right: activeTab = %v; want Input", m.activeTab)
	}
	wheel(tea.MouseWheelLeft, 0)
	if m.activeTab != debugTabRuntime {
		t.Fatalf("wheel-left: activeTab = %v; want Runtime", m.activeTab)
	}
	wheel(tea.MouseWheelDown, tea.ModShift)
	if m.activeTab != debugTabInput {
		t.Fatalf("shift+wheel-down: activeTab = %v; want Input", m.activeTab)
	}
	wheel(tea.MouseWheelUp, tea.ModShift)
	if m.activeTab != debugTabRuntime {
		t.Fatalf("shift+wheel-up: activeTab = %v; want Runtime", m.activeTab)
	}

	// Plain vertical wheel on a table tab moves the row cursor, not the tab.
	before := m.runtimeTbl.Cursor()
	wheel(tea.MouseWheelDown, 0)
	if m.activeTab != debugTabRuntime {
		t.Fatalf("plain wheel-down must not switch tabs; activeTab = %v", m.activeTab)
	}
	if got := m.runtimeTbl.Cursor(); got != before+1 {
		t.Fatalf("plain wheel-down: table cursor = %d; want %d", got, before+1)
	}
}

// unstyledGapRE matches an SGR reset followed by bare spaces — a run of cells
// that lost the row background between two styled segments.
var unstyledGapRE = regexp.MustCompile(`\x1b\[0?m +\x1b`)

// TestSettingsSelectedRowHasNoBackgroundGaps is the regression test for the
// inspector Settings tab highlight: the indicator, field, separator, and value
// must form one continuous selection-background bar, matching the main
// settings page (no unstyled spaces between the columns).
func TestSettingsSelectedRowHasNoBackgroundGaps(t *testing.T) {
	t.Parallel()

	m := New()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m.switchTab(debugTabSettings)

	out := m.renderSettingsSection(theme.Active())
	checked := false
	for line := range strings.SplitSeq(out, "\n") {
		if !strings.Contains(line, "▶") && !strings.Contains(line, "↵") {
			continue
		}
		if !strings.Contains(line, "\x1b[") {
			t.Skip("no ANSI styling emitted — cannot verify backgrounds")
		}
		checked = true
		if loc := unstyledGapRE.FindString(line); loc != "" {
			t.Fatalf("selected row has an unstyled background gap between columns: %q", line)
		}
	}
	if !checked {
		t.Fatal("no selected row found in the settings section render")
	}
}
