package settings

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jarvisfriends/tui-base/config"
)

func newSP9Model(t *testing.T) *SettingsModel {
	t.Helper()
	url := "https://example.com"
	single := "only"
	m := NewWithOptions(Options{ExtraSections: []config.Section[string]{{
		Title: "My App",
		Fields: []config.FieldDef[string]{
			{Kind: config.FieldText, Title: "API URL", Value: &url},
			{Kind: config.FieldText, Title: "Second", Value: &single},
		},
	}}})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	return m
}

// TestSP9FrameworkCategoriesStartCollapsed: framework categories default to
// collapsed; app-provided (ExtraSections) categories stay expanded; collapse
// state survives a buildItems rebuild.
func TestSP9FrameworkCategoriesStartCollapsed(t *testing.T) {
	t.Parallel()

	m := newSP9Model(t)
	for _, cat := range m.categories {
		if cat.appOwned && cat.collapsed {
			t.Errorf("app category %q should start expanded", cat.title)
		}
		if !cat.appOwned && !cat.collapsed {
			t.Errorf("framework category %q should start collapsed", cat.title)
		}
	}

	// Expand one framework category, rebuild, and expect the choice kept.
	var nav int
	for i, cat := range m.categories {
		if cat.title == "Navigation" {
			nav = i
		}
	}
	m.toggleCategory(nav)
	m.buildItems()
	for _, cat := range m.categories {
		if cat.title == "Navigation" && cat.collapsed {
			t.Error("Navigation expansion lost across rebuild")
		}
	}
}

// TestSP9CollapsedItemsHiddenFromOverview: a collapsed category renders only
// its marked header; expanding shows the items again.
func TestSP9CollapsedItemsHiddenFromOverview(t *testing.T) {
	t.Parallel()

	m := newSP9Model(t)
	out := ansi.Strip(m.renderOverview())
	if strings.Contains(out, "Navigation Style") {
		t.Errorf("collapsed Navigation should hide its items:\n%s", out)
	}
	if !strings.Contains(out, "▸ Navigation") {
		t.Errorf("collapsed header should carry the ▸ marker:\n%s", out)
	}
	if !strings.Contains(out, "API URL") {
		t.Errorf("app category items should be visible:\n%s", out)
	}

	m.ExpandAllCategories()
	out = ansi.Strip(m.renderOverview())
	if !strings.Contains(out, "Navigation Style") {
		t.Errorf("expanded Navigation should show its items:\n%s", out)
	}
	if !strings.Contains(out, "▾ Navigation") {
		t.Errorf("expanded header should carry the ▾ marker:\n%s", out)
	}
}

// TestSP9EnterOnHeaderToggles: the cursor starts on the first stop and Enter
// toggles a header's collapse from the keyboard.
func TestSP9EnterOnHeaderToggles(t *testing.T) {
	t.Parallel()

	m := NewWithOptions(Options{})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	if m.headerCursor < 0 {
		t.Fatalf("cursor should start on a header when everything is collapsed; headerCursor=%d",
			m.headerCursor)
	}
	first := m.headerCursor
	if !m.categories[first].collapsed {
		t.Fatal("first framework category should start collapsed")
	}
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.categories[first].collapsed {
		t.Fatal("Enter on the header should expand the category")
	}
	// Down enters the category's first editable item; Enter opens its editor.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.headerCursor != -1 {
		t.Fatalf("Down from header should land on an item; headerCursor=%d", m.headerCursor)
	}
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.editOverlay.IsOpen() && !m.modelOverlay.IsOpen() {
		t.Fatal("Enter on an item should open its editor")
	}
}

// TestSP9SingleChoiceIsDisplayOnly: an item with one effective option is
// skipped by the cursor and refuses to open an editor.
func TestSP9SingleChoiceIsDisplayOnly(t *testing.T) {
	t.Parallel()

	m := newSP9Model(t)
	m.ExpandAllCategories()
	idx := findItemIndex(t, m, "Navigation Style")
	m.items[idx].choices = 1

	for _, s := range m.overviewStops() {
		if !s.isHeader && s.item == idx {
			t.Fatal("display-only item should not be a cursor stop")
		}
	}
	selectItemForTest(m, idx)
	if cmd := m.startEdit(); cmd != nil {
		t.Fatal("display-only item should not open an editor")
	}
	if m.editOverlay.IsOpen() || m.modelOverlay.IsOpen() {
		t.Fatal("no overlay should open for a display-only item")
	}
	// The value still shows in the overview (display-only, not hidden).
	out := ansi.Strip(m.renderOverview())
	if !strings.Contains(out, "Navigation Style") {
		t.Errorf("display-only row should still render:\n%s", out)
	}
}
