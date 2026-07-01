package table

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jarvisfriends/tui-base/theme"
)

func sampleCols() []Column {
	return []Column{{Title: "Name", Filter: true}, {Title: "N"}}
}

func sampleRows() []Row {
	return []Row{
		{Key: "a", Cells: []Cell{Text("Apple"), Num("3", 3)}},
		{Key: "b", Cells: []Cell{Text("Banana"), Num("1", 1)}},
		{Key: "c", Cells: []Cell{Text("Cherry"), Num("2", 2)}},
	}
}

// TestNumericSortDesc verifies numeric cells sort by magnitude (not lexically)
// and that the default descending sort orders highest-first.
func TestNumericSortDesc(t *testing.T) {
	m := New(sampleCols(), WithSort(1, false))
	m.SetRows(sampleRows())
	if got := []string{m.rows[0].Key, m.rows[1].Key, m.rows[2].Key}; got[0] != "a" || got[1] != "c" || got[2] != "b" {
		t.Fatalf("desc numeric sort wrong: %v", got)
	}
	if r, ok := m.SelectedRow(); !ok || r.Key != "a" {
		t.Fatalf("selected row should be the top (a): %+v ok=%v", r, ok)
	}
}

// TestSortByColCycle verifies the 3-state header cycle: asc → desc → unsorted.
func TestSortByColCycle(t *testing.T) {
	m := New(sampleCols())
	m.SetRows(sampleRows())

	m.sortByCol(1)
	if !m.sortActive || !m.sortAsc {
		t.Fatalf("first click should sort ascending")
	}
	m.sortByCol(1)
	if !m.sortActive || m.sortAsc {
		t.Fatalf("second click should sort descending")
	}
	m.sortByCol(1)
	if m.sortActive {
		t.Fatalf("third click should clear the sort")
	}
}

// TestFilter checks the filter selects only rows whose filterable columns match.
func TestFilter(t *testing.T) {
	m := New(sampleCols())
	m.SetRows(sampleRows())
	m.filter = "an" // matches "Banana"
	m.rebuildFilter()
	if len(m.filtered) != 1 {
		t.Fatalf("filter 'an' should match 1 row, got %d", len(m.filtered))
	}
	if r, _ := m.SelectedRow(); r.Key != "b" {
		t.Fatalf("filtered selection should be Banana, got %q", r.Key)
	}
}

// TestOpenEmitsMsg verifies Enter emits an OpenDetailMsg for the selected row.
func TestOpenEmitsMsg(t *testing.T) {
	m := New(sampleCols(), WithSort(1, false))
	m.SetRows(sampleRows())
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should return a command")
	}
	msg, ok := cmd().(OpenDetailMsg)
	if !ok {
		t.Fatalf("expected OpenDetailMsg, got %T", cmd())
	}
	if msg.Key != "a" {
		t.Fatalf("OpenDetailMsg should carry the selected key (a), got %q", msg.Key)
	}
}

// TestKeyboardSortKey drives the sort key through Update (the real input path),
// constructing the KeyPressMsg the way the terminal does (Text set), and checks
// the sort actually changes.
func TestKeyboardSortKey(t *testing.T) {
	m := New(sampleCols())
	m.SetRows(sampleRows())
	if m.sortActive {
		t.Fatal("table should start unsorted")
	}
	if cmd := m.Update(tea.KeyPressMsg{Text: "s"}); cmd != nil {
		t.Errorf("sort key should not return a command, got %T", cmd)
	}
	if !m.sortActive || m.sortCol != 0 || !m.sortAsc {
		t.Fatalf("'s' should sort column 0 ascending; got active=%v col=%d asc=%v", m.sortActive, m.sortCol, m.sortAsc)
	}
	m.Update(tea.KeyPressMsg{Text: "s"})
	if !m.sortActive || m.sortAsc {
		t.Fatalf("second 's' should flip to descending; got active=%v asc=%v", m.sortActive, m.sortAsc)
	}
}

// TestKeyboardSortReordersRows confirms a keyboard sort actually reorders the
// underlying rows, not just the sort flags.
func TestKeyboardSortReordersRows(t *testing.T) {
	m := New(sampleCols())
	m.SetRows(sampleRows())
	m.Update(tea.KeyPressMsg{Text: "s"}) // sort column 0 (Name) ascending
	if m.rows[0].Key != "a" || m.rows[2].Key != "c" {
		t.Fatalf("ascending name sort wrong order: %s,%s,%s", m.rows[0].Key, m.rows[1].Key, m.rows[2].Key)
	}
}

// TestMouseHeaderSort clicks a column header (after a render records geometry)
// and checks the 3-state cycle runs.
func TestMouseHeaderSort(t *testing.T) {
	m := New(sampleCols())
	m.SetRows(sampleRows())
	m.SetSize(60, 20)
	_ = m.View(theme.Active(), 1) // records headerY + column boundaries

	if len(m.colBoundaries) == 0 {
		t.Fatal("no column boundaries recorded; cannot locate header columns")
	}
	x := m.colBoundaries[0] + 1 // a point inside column 1

	m.HandleClick(x, m.headerY)
	if !m.sortActive || m.sortCol != 1 || !m.sortAsc {
		t.Fatalf("header click should sort col 1 ascending; got active=%v col=%d asc=%v", m.sortActive, m.sortCol, m.sortAsc)
	}
	m.HandleClick(x, m.headerY)
	if !m.sortActive || m.sortAsc {
		t.Fatalf("second header click should be descending; got active=%v asc=%v", m.sortActive, m.sortAsc)
	}
	m.HandleClick(x, m.headerY)
	if m.sortActive {
		t.Fatal("third header click should clear the sort")
	}
}

// TestMouseDoubleClickOpens checks a quick second click on a row opens details.
func TestMouseDoubleClickOpens(t *testing.T) {
	m := New(sampleCols(), WithSort(1, false))
	m.SetRows(sampleRows())
	m.SetSize(60, 20)
	_ = m.View(theme.Active(), 0)

	y := m.dataStartY // first data row
	if cmd := m.HandleClick(3, y); cmd != nil {
		t.Fatal("a single click should select, not open details")
	}
	cmd := m.HandleClick(3, y) // immediate second click → double-click
	if cmd == nil {
		t.Fatal("double-click should open details")
	}
	if _, ok := cmd().(OpenDetailMsg); !ok {
		t.Fatalf("double-click should emit OpenDetailMsg, got %T", cmd())
	}
}

// TestViewRecordsGeometry renders the table and checks it records the geometry
// HandleClick relies on (column boundaries + data row origin).
func TestViewRecordsGeometry(t *testing.T) {
	m := New(sampleCols(), WithSort(1, false))
	m.SetRows(sampleRows())
	m.SetSize(60, 20)

	out := m.View(theme.Active(), 1)
	if out == "" {
		t.Fatal("View returned empty output")
	}
	if m.dataStartY != 4 { // originY(1) + top border + header + separator
		t.Errorf("dataStartY = %d, want 4", m.dataStartY)
	}
	if len(m.colBoundaries) != len(sampleCols())-1 {
		t.Errorf("expected %d column boundaries, got %d", len(sampleCols())-1, len(m.colBoundaries))
	}
}

// TestColumnBoundariesAreScreenColumns confirms colBoundaries are recorded in
// the same coordinate space HandleClick receives from a real mouse: visual
// screen columns on the ANSI-stripped line, not rune indices into the raw
// (color-escaped) render. BorderStyle colors the top border, so the raw
// string carries SGR escape sequences; without stripping them first, every
// recorded boundary is offset by however many escape-sequence runes preceded
// it, and clicks landing on real screen coordinates would silently resolve to
// the wrong column (or none) the moment color output is enabled.
func TestColumnBoundariesAreScreenColumns(t *testing.T) {
	m := New(sampleCols())
	m.SetRows(sampleRows())
	m.SetSize(60, 20)
	out := m.View(theme.Active(), 1)

	top := ansi.Strip(strings.Split(out, "\n")[0])
	topRunes := []rune(top)
	junction, _ := utf8.DecodeRuneInString(tableBorder.MiddleTop)

	if len(m.colBoundaries) == 0 {
		t.Fatal("no column boundaries recorded")
	}
	for _, x := range m.colBoundaries {
		if x < 0 || x >= len(topRunes) {
			t.Fatalf("boundary x=%d out of range for stripped top border (len %d): %q", x, len(topRunes), top)
		}
		if topRunes[x] != junction {
			t.Errorf("boundary x=%d lands on %q in the visual top border, want junction glyph %q: %q",
				x, topRunes[x], junction, top)
		}
	}
}

// TestColumnBoundariesTrackBorderStyle sweeps every standard lipgloss border
// preset through tableBorder and re-verifies header-click sorting still
// works for each one. Column-boundary detection scans the rendered top
// border for tableBorder's own junction glyph; if that ever regresses back
// to a hardcoded literal (e.g. "┬"), this fails the moment the border style
// changes to one with a different junction glyph (Thick "┳", Double "╦"),
// which is exactly the failure mode a future themed-border feature would
// otherwise trigger silently — header clicks would stop sorting anything.
func TestColumnBoundariesTrackBorderStyle(t *testing.T) {
	original := tableBorder
	t.Cleanup(func() { tableBorder = original })

	borders := map[string]lipgloss.Border{
		"normal":  lipgloss.NormalBorder(),
		"rounded": lipgloss.RoundedBorder(),
		"thick":   lipgloss.ThickBorder(),
		"double":  lipgloss.DoubleBorder(),
		"ascii":   lipgloss.ASCIIBorder(),
	}

	for name, b := range borders {
		t.Run(name, func(t *testing.T) {
			tableBorder = b

			m := New(sampleCols())
			m.SetRows(sampleRows())
			m.SetSize(60, 20)
			_ = m.View(theme.Active(), 1)

			if len(m.colBoundaries) != len(sampleCols())-1 {
				t.Fatalf("border=%s: expected %d column boundaries, got %d (junction glyph %q not found in rendered top border)",
					name, len(sampleCols())-1, len(m.colBoundaries), b.MiddleTop)
			}

			x := m.colBoundaries[0] + 1
			m.HandleClick(x, m.headerY)
			if !m.sortActive || m.sortCol != 1 || !m.sortAsc {
				t.Fatalf("border=%s: header click should sort col 1 ascending; got active=%v col=%d asc=%v",
					name, m.sortActive, m.sortCol, m.sortAsc)
			}
		})
	}
}
