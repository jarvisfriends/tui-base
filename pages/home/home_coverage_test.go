package home

import (
	"image/color"
	"testing"

	"github.com/jarvisfriends/snap/styles"
	"github.com/jarvisfriends/snap/table"
	"github.com/jarvisfriends/snap/uifx"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	tint "github.com/lrstanley/bubbletint/v2"
)

// tintWithSource returns the ID of a registered tint that carries an
// attribution link, or "" if none do — used to exercise the theme-source code
// paths (themeSource, themeURLLine, the "Open theme source" menu item).
func tintWithSource() string {
	styles.VerifyRegistry()
	for _, t := range tint.Tints() {
		if len(t.CreditSources) > 0 && t.CreditSources[0].Link != "" {
			return t.ID
		}
	}
	return ""
}

// TestProgressStyleHelpers covers the progress-style/effects logic that the
// load-bar controls drive.
func TestProgressStyleHelpers(t *testing.T) {
	// availableProgressStyles per tier.
	if got := len(availableProgressStyles(uifx.LevelMinimal)); got != 1 {
		t.Errorf("minimal styles = %d, want 1", got)
	}
	if got := len(availableProgressStyles(uifx.LevelMedium)); got != 4 {
		t.Errorf("medium styles = %d, want 4", got)
	}
	if got := len(availableProgressStyles(uifx.LevelHigh)); got != 5 {
		t.Errorf("high styles = %d, want 5", got)
	}

	// String() for every style.
	for _, s := range []progressStyle{progressClassic, progressBlend, progressSolid, progressZones, progressAnimated} {
		if s.String() == "" {
			t.Errorf("progressStyle %d has empty String()", s)
		}
	}

	// cycleEffects walks Minimal → Medium → High → Minimal.
	m := New()
	m.Effects = uifx.LevelMinimal
	m.cycleEffects()
	if m.Effects != uifx.LevelMedium {
		t.Fatalf("after cycle from minimal: %v", m.Effects)
	}
	m.cycleEffects()
	if m.Effects != uifx.LevelHigh {
		t.Fatalf("after cycle from medium: %v", m.Effects)
	}
	m.cycleEffects()
	if m.Effects != uifx.LevelMinimal {
		t.Fatalf("after cycle from high: %v", m.Effects)
	}

	// clampProgressStyle: animated is only valid at High; dropping to Medium
	// snaps it back to classic.
	m.Effects = uifx.LevelHigh
	m.progStyle = progressAnimated
	m.clampProgressStyle()
	if m.progStyle != progressAnimated {
		t.Fatalf("animated should survive at High; got %v", m.progStyle)
	}
	m.Effects = uifx.LevelMedium
	m.clampProgressStyle()
	if m.progStyle != progressClassic {
		t.Fatalf("animated should clamp to classic at Medium; got %v", m.progStyle)
	}

	// cycleProgressStyle wraps through the whole High set.
	m.Effects = uifx.LevelHigh
	m.progStyle = progressClassic
	seen := map[progressStyle]bool{}
	for range availableProgressStyles(uifx.LevelHigh) {
		seen[m.progStyle] = true
		_ = m.cycleProgressStyle()
	}
	if len(seen) != 5 {
		t.Fatalf("cycleProgressStyle visited %d styles, want 5", len(seen))
	}
}

// TestProgressViewAllStyles renders the load bar in every style so each branch
// of progressView executes.
func TestProgressViewAllStyles(t *testing.T) {
	m := sized(t, 100, 40)
	for _, s := range []progressStyle{progressClassic, progressBlend, progressSolid, progressZones, progressAnimated} {
		m.progStyle = s
		if got := m.progressView(20); got == "" {
			t.Errorf("progressView(%v) is empty", s)
		}
	}
	// Render the animated bar with a non-zero fill so its per-cell color
	// function (from newAnimProgress) actually runs.
	if got := m.animProgress.ViewAs(0.8); got == "" {
		t.Error("animated bar render is empty")
	}
}

// TestContextItemsEveryZone builds the right-click menu for every zone.
func TestContextItemsEveryZone(t *testing.T) {
	if id := tintWithSource(); id != "" {
		prev := currentTintID()
		t.Cleanup(func() { _ = styles.SetCurrentTint(prev) })
		_ = styles.SetCurrentTint(id)
	}
	m := sized(t, 100, 40)
	for _, zone := range []string{
		zoneBox, zonePills, zoneCharts, zoneProgressStyle,
		zoneTheme, zoneThemeURL, zoneEffects, zoneResponsive,
	} {
		if items := m.contextItems(zone); len(items) == 0 {
			t.Errorf("contextItems(%q) returned no items", zone)
		}
	}
	if m.contextItems("nonexistent") != nil {
		t.Error("unknown zone should yield nil items")
	}
}

// TestApplyContextChoiceEveryAction exercises every context-menu action ID.
func TestApplyContextChoiceEveryAction(t *testing.T) {
	m := sized(t, 100, 40)

	shape0 := m.shape
	m.applyContextChoice("cycle-shape")
	if m.shape == shape0 {
		t.Error("cycle-shape did not change shape")
	}

	m.applyContextChoice("toggle-pause")
	if !m.paused {
		t.Error("toggle-pause did not pause")
	}

	spark0 := m.sparkStyle
	m.applyContextChoice("cycle-spark-style")
	if m.sparkStyle == spark0 {
		t.Error("cycle-spark-style did not change style")
	}

	m.Effects = uifx.LevelHigh
	prog0 := m.progStyle
	m.applyContextChoice("cycle-progress-style")
	if m.progStyle == prog0 {
		t.Error("cycle-progress-style did not change style")
	}

	fx0 := m.Effects
	m.applyContextChoice("cycle-effects")
	if m.Effects == fx0 {
		t.Error("cycle-effects did not change level")
	}

	// cycle-theme returns a ThemeMsg-producing command.
	if cmd := m.applyContextChoice("cycle-theme"); cmd == nil {
		t.Error("cycle-theme should return a command")
	} else {
		_ = cmd() // executing just yields the message; no side effects
	}

	// open-theme-source returns a command only when the theme has a source.
	_ = m.applyContextChoice(actionOpenThemeSource)

	// Disk actions: Open returns a (non-executed) command; Refresh re-enumerates.
	m.menuDiskPath = driveC
	if cmd := m.applyContextChoice(diskActionOpen); cmd == nil {
		t.Error("disk open should return a command")
	}
	m.applyContextChoice(diskActionRefresh)

	// An unknown ID is a no-op.
	if m.applyContextChoice("nope") != nil {
		t.Error("unknown action should return nil")
	}
}

// TestHomeLeftClickZones drives left clicks on each interactive strip.
func TestHomeLeftClickZones(t *testing.T) {
	m := sized(t, 100, 40)
	leftClickZone := func(zone string) {
		b, ok := m.zones.Bounds(zone)
		if !ok {
			t.Fatalf("zone %q not registered", zone)
		}
		_ = m.View().OnMouse(tea.MouseClickMsg(tea.Mouse{X: b.X + 1, Y: b.Y, Button: tea.MouseLeft}))
	}

	prog0 := m.progStyle
	leftClickZone(zoneProgressStyle)
	if m.progStyle == prog0 {
		t.Error("clicking progress-style strip should cycle it")
	}

	fx0 := m.Effects
	leftClickZone(zoneEffects)
	if m.Effects == fx0 {
		t.Error("clicking effects strip should cycle it")
	}

	// Theme strip returns a command via onClick; just verify it doesn't panic.
	leftClickZone(zoneTheme)
}

// TestHomeRightClickOpensMenu covers the right-click → context-menu path and
// the menu-open branch of the keyboard/mouse routers.
func TestHomeRightClickOpensMenu(t *testing.T) {
	m := sized(t, 100, 40)
	b, ok := m.zones.Bounds(zonePills)
	if !ok {
		t.Fatal("pills zone missing")
	}
	_ = m.View().OnMouse(tea.MouseClickMsg(tea.Mouse{X: b.X + 1, Y: b.Y, Button: tea.MouseRight}))
	if !m.contextMenu.IsOpen() {
		t.Fatal("right-click should open the context menu")
	}

	// With the menu open, a key press is routed to the menu (Update branch).
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	// A mouse event with the menu open is routed to it (onMouseRoot branch).
	_ = m.View().OnMouse(tea.MouseMotionMsg(tea.Mouse{X: b.X + 1, Y: b.Y}))

	// Enter chooses the highlighted item and closes the menu.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// TestHomeRightClickDisksRow covers onClick's disks right-click branch: it
// selects the row under the pointer and opens that drive's actions menu.
func TestHomeRightClickDisksRow(t *testing.T) {
	m := New()
	m.disks = sampleDisks()
	m.disksTbl.SetRows(diskRows(m.disks))
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	_ = m.View()
	if m.vp.YOffset() != 0 {
		t.Skip("content scrolled; screen≠content coords")
	}
	// Right-click the first data row (one below the table header).
	_ = m.View().OnMouse(tea.MouseClickMsg(tea.Mouse{X: m.disksLeft + 1, Y: m.disksTop + 1, Button: tea.MouseRight}))
	if !m.contextMenu.IsOpen() {
		t.Fatal("right-click on a disk row should open the drive menu")
	}
	if m.menuDiskPath == "" {
		t.Error("a drive path should be recorded for the opened menu")
	}
}

// TestHomeWheelAndMotion covers onWheel (over the disks table vs. the page) and
// onMotion hover tracking at LevelHigh (which drives hoverWrap).
func TestHomeWheelAndMotion(t *testing.T) {
	m := New()
	m.Effects = uifx.LevelHigh
	m.disks = sampleDisks()
	m.disksTbl.SetRows(diskRows(m.disks))
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	_ = m.View()

	// Wheel over the disks table moves its selection.
	if db, ok := m.zones.Bounds(zoneDisks); ok {
		_ = m.View().OnMouse(tea.MouseWheelMsg(tea.Mouse{X: db.X + 1, Y: db.Y + 2, Button: tea.MouseWheelDown}))
	}
	// Wheel elsewhere scrolls the page viewport.
	_ = m.View().OnMouse(tea.MouseWheelMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseWheelUp}))

	// Motion over a zone at LevelHigh records the hover zone; the next render
	// wraps that block via hoverWrap.
	if pb, ok := m.zones.Bounds(zonePills); ok {
		_ = m.View().OnMouse(tea.MouseMotionMsg(tea.Mouse{X: pb.X + 1, Y: pb.Y}))
		if m.hoverZone != zonePills {
			t.Errorf("hoverZone = %q, want %q", m.hoverZone, zonePills)
		}
	}
	_ = m.View() // renders with hover border (hoverWrap)
}

// TestHomeUpdatePaths covers the non-mouse Update branches.
func TestHomeUpdatePaths(t *testing.T) {
	m := sized(t, 100, 40)

	// progress.FrameMsg drives the animated bar model.
	_, _ = m.Update(progress.FrameMsg{})

	// browserOpenedMsg with an error hits the logging branch.
	_, _ = m.Update(browserOpenedMsg{URL: "x", Err: stubError{}})

	// A disks OpenDetailMsg opens the drive actions menu.
	_, _ = m.Update(table.OpenDetailMsg{Key: driveC})
	if !m.contextMenu.IsOpen() {
		t.Error("OpenDetailMsg should open the disk menu")
	}
	m.contextMenu.Close()

	// A key the disks table owns is forwarded to it; an unowned key falls
	// through to the viewport.
	_, _ = m.Update(tea.KeyPressMsg{Code: 's'})
	_, _ = m.Update(tea.KeyPressMsg{Code: 'x'})
}

// stubError is a trivial error for exercising error branches.
type stubError struct{}

func (stubError) Error() string { return "stub" }

// TestDisksHelpAndFilter covers ShortHelp/FullHelp (both the normal and
// filtering variants), CapturesKeys, and openDiskMenu edge cases.
func TestDisksHelpAndFilter(t *testing.T) {
	m := sized(t, 100, 40)

	if len(m.ShortHelp()) == 0 || len(m.FullHelp()) == 0 {
		t.Fatal("help bindings should be non-empty")
	}
	if m.CapturesKeys() {
		t.Error("should not capture keys before filtering")
	}

	// Enter the table's filter mode; the help collapses and keys are captured.
	_ = m.disksTbl.Update(tea.KeyPressMsg{Code: '/'})
	if !m.disksTbl.Filtering() {
		t.Skip("table did not enter filter mode")
	}
	if !m.CapturesKeys() {
		t.Error("filtering should capture keys")
	}
	if len(m.ShortHelp()) == 0 || len(m.FullHelp()) == 0 {
		t.Error("filter-mode help should be non-empty")
	}
	if !m.disksHandlesKey(tea.KeyPressMsg{Code: 'q'}) {
		t.Error("every key belongs to the filter while filtering")
	}

	// openDiskMenu ignores an empty path and records a real one.
	m.openDiskMenu(5, 5, "")
	m.disks = sampleDisks()
	m.openDiskMenu(5, 5, driveC)
	if m.menuDiskPath != driveC {
		t.Errorf("menuDiskPath = %q, want %q", m.menuDiskPath, driveC)
	}
}

// TestHomeMiscHelpers covers small standalone helpers.
func TestHomeMiscHelpers(t *testing.T) {
	m := New()
	if m.Init() != nil {
		t.Error("Init should return nil")
	}

	// blendAt clamps and samples the ramp.
	from, to := color.RGBA{R: 0, A: 255}, color.RGBA{R: 255, A: 255}
	for _, tt := range []float64{-1, 0, 0.5, 1, 2} {
		if blendAt(from, to, tt) == nil {
			t.Errorf("blendAt(%.1f) returned nil", tt)
		}
	}

	// themeSource / themeURLLine over a sourced tint.
	id := tintWithSource()
	if id == "" {
		return
	}
	prev := currentTintID()
	t.Cleanup(func() { _ = styles.SetCurrentTint(prev) })
	_ = styles.SetCurrentTint(id)
	if _, _, ok := m.themeSource(); !ok {
		return
	}
	if m.themeURLLine() == "" {
		t.Error("themeURLLine should be non-empty for a sourced tint")
	}
	if m.openThemeSourceCmd() == nil {
		t.Error("openThemeSourceCmd should return a command for a sourced tint")
	}
}

// currentTintID returns the active tint ID (or "" if the registry is empty).
func currentTintID() string {
	styles.VerifyRegistry()
	if c := styles.Active(); c.OrigTint != nil {
		return c.OrigTint.ID
	}
	return ""
}
