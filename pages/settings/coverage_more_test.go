// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"

	"github.com/jarvisfriends/snap/datepicker"
	"github.com/jarvisfriends/snap/pickers"
	"github.com/jarvisfriends/snap/timepicker"
	"github.com/jarvisfriends/tui-base/config"
)

// newTestModel builds a settings model persisted under an isolated dir and
// gives it a workable size. Not parallel-safe: SetConfigDir is package state.
func newTestModel(t *testing.T, opts Options) *SettingsModel {
	t.Helper()
	SetConfigDir(t.TempDir())
	t.Cleanup(func() { SetConfigDir("") })
	t.Setenv("TMPDIR", t.TempDir()) // keep default-mode log files inside the test dir
	m := NewWithOptions(opts)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

// nudgeMsg is a no-op message used to flush overlay state checks without
// perturbing the hosted model (arrow keys would move pickers around).
type nudgeMsg struct{}

// pump runs a command tree like a miniature program loop: every produced
// message is offered to fn and fed back into the model, and follow-up
// commands keep running until the queue drains (bounded for safety).
func pump(m *SettingsModel, cmd tea.Cmd, fn func(tea.Msg)) {
	queue := []tea.Cmd{cmd}
	for steps := 0; steps < 200 && len(queue) > 0; steps++ {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, b := range batch {
				queue = append(queue, b)
			}
			continue
		}
		if fn != nil {
			fn(msg)
		}
		_, next := m.Update(msg)
		queue = append(queue, next)
	}
}

// runCmds executes a command tree, feeding nothing back; fn (optional) sees
// every produced message.
func runCmds(cmd tea.Cmd, fn func(tea.Msg)) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			runCmds(c, fn)
		}
		return
	}
	if msg != nil && fn != nil {
		fn(msg)
	}
}

// openItem points the cursor at the titled item and presses Enter.
func openItem(t *testing.T, m *SettingsModel, title string) {
	t.Helper()
	selectItemForTest(m, findItemIndex(t, m, title))
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.CapturesKeys() {
		t.Fatalf("opening %q did not start an edit", title)
	}
}

// TestCompleteSelectEditSavesAndBroadcasts drives a full select edit: the
// value changes live (NavStyleMsg), Enter completes the form, and the
// completed path saves the file and emits the runtime messages.
func TestCompleteSelectEditSavesAndBroadcasts(t *testing.T) {
	m := newTestModel(t, Options{})

	openItem(t, m, "Navigation Style")
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // live change fires NavStyleMsg
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	var sawNotif, sawKeys, sawSaved bool
	pump(m, cmd, func(msg tea.Msg) {
		switch v := msg.(type) {
		case NotificationsSettingsMsg:
			sawNotif = true
		case KeybindingsChangedMsg:
			sawKeys = true
		case SettingsSavedMsg:
			sawSaved = true
			if v.Err != nil {
				t.Errorf("async save failed: %v", v.Err)
			}
		}
	})
	if !sawNotif || !sawKeys || !sawSaved {
		t.Fatalf("completed edit messages: notif=%v keys=%v saved=%v", sawNotif, sawKeys, sawSaved)
	}
	if m.CapturesKeys() {
		t.Fatal("edit overlay should be closed after completion")
	}
	if _, err := os.Stat(FilePath()); err != nil {
		t.Fatalf("settings file not written: %v", err)
	}
}

// TestEditToggleMessages: the nav-numbers and number-select selects emit
// their runtime toggle messages, and a log-level change reconfigures logging.
func TestEditToggleMessages(t *testing.T) {
	m := newTestModel(t, Options{})

	assertEmits := func(title string, want func(tea.Msg) bool) {
		t.Helper()
		openItem(t, m, title)
		_, cmd1 := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		_, cmd2 := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		found := false
		check := func(msg tea.Msg) {
			if want(msg) {
				found = true
			}
		}
		pump(m, cmd1, check)
		pump(m, cmd2, check)
		if !found {
			t.Errorf("%s edit did not emit its runtime message", title)
		}
	}

	assertEmits("Show Nav Numbers", func(msg tea.Msg) bool {
		_, ok := msg.(NavShowNumbersMsg)
		return ok
	})
	assertEmits("Number Key Select", func(msg tea.Msg) bool {
		_, ok := msg.(NavNumberSelectMsg)
		return ok
	})
	assertEmits("Color Theme", func(msg tea.Msg) bool {
		_, ok := msg.(ThemeMsg)
		return ok
	})

	// Log level: the live change funnels through logging.SetLevel.
	openItem(t, m, "Log Level")
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	pump(m, cmd, nil)
	if m.CapturesKeys() {
		t.Fatal("log level edit should be closed")
	}
}

// TestAbortViaCtrlC: huh's own abort key runs the same revert path as Esc.
func TestAbortViaCtrlC(t *testing.T) {
	m := newTestModel(t, Options{})

	openItem(t, m, "Navigation Style")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	pump(m, cmd, nil)
	if m.CapturesKeys() {
		t.Fatal("ctrl+c should abort the edit")
	}
}

// TestLogPathEditorPerDestination: the Log Path item opens nothing for the
// temp destination, a directory browser for "dir", and a file picker form
// for "file"; completing the dir browser writes the value through.
func TestLogPathEditorPerDestination(t *testing.T) {
	m := newTestModel(t, Options{})

	// Temp destination: nothing to edit.
	m.LogOutput = logOutputTemp
	selectItemForTest(m, findItemIndex(t, m, itemTitleLogPath))
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.CapturesKeys() {
		t.Fatal("temp destination should not open an editor")
	}

	// File destination: the huh file-picker form.
	m.LogOutput = logOutputFile
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.editOverlay.IsOpen() {
		t.Fatal("file destination should open the picker form")
	}
	runCmds(m.abortEdit(), nil)

	// Dir destination: the DirPicker model overlay; Done commits the value.
	logDir := t.TempDir()
	m.LogOutput = logOutputDir
	m.LogPath = logDir
	selectItemForTest(m, findItemIndex(t, m, itemTitleLogPath))
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.modelOverlay.IsOpen() {
		t.Fatal("dir destination should open the DirPicker overlay")
	}
	dp, ok := m.modelOverlay.Model().(*pickers.DirPicker)
	if !ok {
		t.Fatalf("hosted model is %T, want *pickers.DirPicker", m.modelOverlay.Model())
	}
	dp.Done = true
	_, cmd := m.Update(nudgeMsg{})
	pump(m, cmd, nil)
	if m.modelOverlay.IsOpen() {
		t.Fatal("completed DirPicker should close the overlay")
	}
}

// TestKeyRecorderOverlayCompleteAndAbort drives the keybinding recorder
// through the model-overlay switch: Done commits, Aborted reverts.
func TestKeyRecorderOverlayCompleteAndAbort(t *testing.T) {
	m := newTestModel(t, Options{})

	openItem(t, m, "Quit Application")
	kr, ok := m.modelOverlay.Model().(*KeyRecorder)
	if !ok {
		t.Fatalf("hosted model is %T, want *KeyRecorder", m.modelOverlay.Model())
	}
	kr.Done = true
	_, cmd := m.Update(nudgeMsg{})
	pump(m, cmd, nil)
	if m.modelOverlay.IsOpen() {
		t.Fatal("Done recorder should close the overlay")
	}
	if _, exists := m.CustomKeys["Quit"]; !exists {
		t.Fatal("recorder value was not written to CustomKeys")
	}

	openItem(t, m, "Quit Application")
	kr, ok = m.modelOverlay.Model().(*KeyRecorder)
	if !ok {
		t.Fatal("recorder overlay missing on reopen")
	}
	kr.Aborted = true
	_, cmd = m.Update(nudgeMsg{})
	pump(m, cmd, nil)
	if m.modelOverlay.IsOpen() {
		t.Fatal("Aborted recorder should close the overlay")
	}
}

// doneAbortModel exercises the generic IsDone/IsAborted duck-typed overlay
// branch for FieldCustom models.
type doneAbortModel struct {
	done, aborted bool
}

func (m *doneAbortModel) Init() tea.Cmd                       { return nil }
func (m *doneAbortModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m *doneAbortModel) View() tea.View                      { return tea.NewView("custom") }
func (m *doneAbortModel) IsDone() bool                        { return m.done }
func (m *doneAbortModel) IsAborted() bool                     { return m.aborted }

// TestExtraSectionModelOverlays covers the multi-file, duration, date, and
// custom model overlays end to end (open → complete/abort → value write).
func TestExtraSectionModelOverlays(t *testing.T) {
	const (
		multiTitle  = "Source List"
		customTitle = "Custom Widget"
	)
	multi, dur, date, custom := "a;b", "1h2m0s", "2024-05-06", "x"
	stub := &doneAbortModel{}
	m := newTestModel(t, Options{ExtraSections: []config.Section[string]{{
		Title: "Overlay Section",
		Fields: []config.FieldDef[string]{
			{Kind: config.FieldMultiFilePicker, Title: multiTitle, Value: &multi},
			{Kind: config.FieldDuration, Title: "Interval", Value: &dur},
			{Kind: config.FieldDate, Title: "Start Date", Value: &date},
			{
				Kind: config.FieldCustom, Title: customTitle, Value: &custom,
				CustomFieldText:    "opens a widget",
				CustomModelBuilder: func() tea.Model { return stub },
			},
		},
	}}})

	// Multi-file editor: Done commits the edited value.
	openItem(t, m, multiTitle)
	mf, ok := m.modelOverlay.Model().(*pickers.MultiFileEditor)
	if !ok {
		t.Fatalf("hosted model is %T, want *pickers.MultiFileEditor", m.modelOverlay.Model())
	}
	mf.Done = true
	_, cmd := m.Update(nudgeMsg{})
	pump(m, cmd, nil)
	if m.modelOverlay.IsOpen() {
		t.Fatal("done multi-file editor should close")
	}

	// Multi-file abort.
	openItem(t, m, multiTitle)
	if mf, ok = m.modelOverlay.Model().(*pickers.MultiFileEditor); !ok {
		t.Fatal("multi-file overlay missing on reopen")
	}
	mf.Aborted = true
	_, cmd = m.Update(nudgeMsg{})
	pump(m, cmd, nil)
	if m.modelOverlay.IsOpen() {
		t.Fatal("aborted multi-file editor should close")
	}

	// Duration spinner: Done writes the duration string back.
	openItem(t, m, "Interval")
	tp, ok := m.modelOverlay.Model().(*timepicker.TimePickerModel)
	if !ok {
		t.Fatalf("hosted model is %T, want *timepicker.TimePickerModel", m.modelOverlay.Model())
	}
	tp.Done = true
	_, cmd = m.Update(nudgeMsg{})
	pump(m, cmd, nil)
	if dur != tp.Duration.String() {
		t.Errorf("duration value = %q, want %q", dur, tp.Duration.String())
	}

	// Duration abort.
	openItem(t, m, "Interval")
	if tp, ok = m.modelOverlay.Model().(*timepicker.TimePickerModel); !ok {
		t.Fatal("duration overlay missing on reopen")
	}
	tp.Aborted = true
	_, cmd = m.Update(nudgeMsg{})
	pump(m, cmd, nil)

	// Date picker: Selected commits, its quit key aborts.
	openItem(t, m, "Start Date")
	dp, ok := m.modelOverlay.Model().(*datepicker.DatePickerModel)
	if !ok {
		t.Fatalf("hosted model is %T, want *datepicker.DatePickerModel", m.modelOverlay.Model())
	}
	dp.Selected = true
	dp.Time = time.Date(2025, 6, 7, 0, 0, 0, 0, time.UTC)
	_, cmd = m.Update(nudgeMsg{})
	pump(m, cmd, nil)
	if date != "2025-06-07" {
		t.Errorf("date value = %q, want 2025-06-07", date)
	}

	openItem(t, m, "Start Date")
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}) // datepicker quit key
	runCmds(cmd, nil)
	if m.modelOverlay.IsOpen() {
		t.Fatal("datepicker quit key should abort the edit")
	}

	// Custom model: generic IsDone, then IsAborted.
	openItem(t, m, customTitle)
	stub.done = true
	_, cmd = m.Update(nudgeMsg{})
	pump(m, cmd, nil)
	if m.modelOverlay.IsOpen() {
		t.Fatal("done custom model should close")
	}
	stub.done = false
	openItem(t, m, customTitle)
	stub.aborted = true
	_, cmd = m.Update(nudgeMsg{})
	pump(m, cmd, nil)
	if m.modelOverlay.IsOpen() {
		t.Fatal("aborted custom model should close")
	}
}

// TestViewMouseOverviewClicks drives the overview's OnMouse hit-testing:
// header toggles, item clicks that open editors, display-only rows, and the
// out-of-bounds guards.
func TestViewMouseOverviewClicks(t *testing.T) {
	m := newTestModel(t, Options{})
	m.ExpandAllCategories()
	m.headerCursor = -1
	m.cursor = 0

	v := m.View()
	layout := m.overviewLayout()

	entryY := func(entryIdx int) int { return layout.listTopY + entryIdx - m.scrollTop }

	// Entry 0 is a category header: clicking collapses it.
	if !layout.entries[0].isHeader {
		t.Fatal("expected the first entry to be a header")
	}
	wasCollapsed := m.categories[layout.entries[0].catIndex].collapsed
	_ = v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: 1, Y: entryY(0)}))
	if m.categories[layout.entries[0].catIndex].collapsed == wasCollapsed {
		t.Fatal("header click should toggle the category")
	}
	_ = v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: 1, Y: entryY(0)})) // expand again

	// Entry 1 is the first item: clicking opens its editor.
	layout = m.overviewLayout()
	if layout.entries[1].isHeader {
		t.Fatal("expected entry 1 to be an item")
	}
	v = m.View()
	_ = v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: 1, Y: entryY(1)}))
	if !m.CapturesKeys() {
		t.Fatal("item click should open its editor")
	}

	// With the edit overlay open: inside clicks are consumed, non-clicks are
	// ignored, outside clicks abort.
	v = m.View() // records the overlay bounds
	b := m.editOverlay.Bounds()
	if cmd := v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: b.X + 1, Y: b.Y + 1})); cmd != nil {
		t.Error("clicks inside the form are consumed without action")
	}
	if cmd := v.OnMouse(tea.MouseMotionMsg(tea.Mouse{X: 0, Y: 0})); cmd != nil {
		t.Error("non-click mouse over the form overlay is ignored")
	}
	runCmds(v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0})), nil)
	if m.CapturesKeys() {
		t.Fatal("outside click should abort the edit")
	}

	// Out-of-bounds guards.
	v = m.View()
	layout = m.overviewLayout()
	if cmd := v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: 1, Y: 0})); cmd != nil {
		t.Error("clicks above the list are ignored")
	}
	if cmd := v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: layout.colWidth, Y: layout.listTopY})); cmd != nil {
		t.Error("clicks in the column gap are ignored")
	}
	gapX := layout.columns * (layout.colWidth + layout.gap)
	if cmd := v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: gapX + 1, Y: layout.listTopY})); cmd != nil {
		t.Error("clicks right of the last column are ignored")
	}
	if cmd := v.OnMouse(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown})); cmd != nil {
		t.Error("wheel is not a click: OnMouse ignores it")
	}

	// A display-only row (one effective option) neither opens nor moves focus.
	itemIdx := layout.entries[1].itemIndex
	m.items[itemIdx].choices = 1
	v = m.View() // renders the dimmed display-only row
	if cmd := v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: 1, Y: entryY(1)})); cmd != nil {
		t.Error("display-only rows are not clickable")
	}
	selectItemForTest(m, itemIdx)
	if cmd := m.startEdit(); cmd != nil {
		t.Error("display-only rows are not editable")
	}
	m.items[itemIdx].choices = 0
}

// TestModelOverlayMouseForwarding: outside clicks abort a hosted model
// overlay, everything else is forwarded with translated coordinates.
func TestModelOverlayMouseForwarding(t *testing.T) {
	m := newTestModel(t, Options{})
	openItem(t, m, "Quit Application")

	v := m.View() // records overlay bounds
	// Motion inside is forwarded to the hosted model (which ignores it).
	_ = v.OnMouse(tea.MouseMotionMsg(tea.Mouse{X: 2, Y: 2}))
	// Outside click aborts.
	runCmds(v.OnMouse(tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0})), nil)
	if m.modelOverlay.IsOpen() {
		t.Fatal("outside click should abort the model overlay")
	}
}

func TestRenderOverviewSpecialRows(t *testing.T) {
	m := newTestModel(t, Options{})
	m.ExpandAllCategories()

	// A long log path exercises the tail-preserving truncation.
	m.LogPath = filepath.Join(strings.Repeat("deep/", 40), "app.log")
	// The header cursor renders with the selection style.
	m.headerCursor = 0
	if got := m.View().Content; got == "" {
		t.Fatal("overview should render")
	}
	m.headerCursor = -1

	// toggleCategory guards and cursor adoption.
	m.toggleCategory(-1)
	m.toggleCategory(len(m.categories)) // out of range: no-op
	m.cursor = m.categories[0].itemIdxSet[0]
	m.toggleCategory(0) // collapsing the cursor's category adopts the header
	if m.headerCursor != 0 {
		t.Fatal("collapsing the cursor's category should move the cursor to its header")
	}
	m.toggleCategory(0)

	// visibleEntries bounds handling.
	var empty overviewLayout
	if got := empty.visibleEntries(0); got != nil {
		t.Errorf("empty layout entries = %v", got)
	}
	full := m.overviewLayout()
	if got := full.visibleEntries(len(full.entries) + 5); got != nil {
		t.Errorf("past-the-end scrollTop entries = %v", got)
	}

	// ensureCursorVisible with no items at all.
	m.items = nil
	m.categories = nil
	m.cursor = 7
	m.ensureCursorVisible()
	if m.cursor != 0 || m.scrollTop != 0 {
		t.Errorf("empty overview cursor=%d scrollTop=%d", m.cursor, m.scrollTop)
	}
}

func TestSaveErrorPaths(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	m := newTestModel(t, Options{})

	// SaveToFile: parent cannot be created.
	if err := m.SaveToFile(filepath.Join(blocker, "sub", "s.json")); err == nil {
		t.Error("SaveToFile into an uncreatable dir should fail")
	}

	// Save: same failure through the sync path.
	SetConfigDir(filepath.Join(blocker, "sub"))
	if err := m.Save(); err == nil {
		t.Error("Save into an uncreatable dir should fail")
	}

	// saveCmd: the async write reports the failure in SettingsSavedMsg.
	msg := m.saveCmd()()
	saved, ok := msg.(SettingsSavedMsg)
	if !ok || saved.Err == nil {
		t.Errorf("async save into an uncreatable dir = %#v; want an error", msg)
	}

	// ReloadFromDisk with no file on disk is a quiet no-op.
	SetConfigDir(filepath.Join(dir, "empty"))
	if changed, cmd := m.ReloadFromDisk(); changed || cmd != nil {
		t.Error("reload without a file should be a no-op")
	}
}

func TestHelpersAndNilReceivers(t *testing.T) {
	opts := []huh.Option[string]{huh.NewOption("Label", "val")}
	if got := labelFor("val", opts); got != "Label" {
		t.Errorf("labelFor hit = %q", got)
	}
	if got := labelFor("zz", opts); got != "zz" {
		t.Errorf("labelFor miss = %q", got)
	}
	if got := tintDisplayName("no-such-tint-id"); got != "no-such-tint-id" {
		t.Errorf("tintDisplayName miss = %q", got)
	}

	var nilKeys *Keys
	if nilKeys.ShortHelp() != nil || nilKeys.FullHelp() != nil {
		t.Error("nil Keys help should be nil")
	}
	var nilModel *SettingsModel
	if nilModel.ShortHelp() != nil || nilModel.FullHelp() != nil {
		t.Error("nil SettingsModel help should be nil")
	}

	// applyTerminalSetting maps every option key (winterm errors off Windows
	// are expected and ignored); unknown keys are always a nil no-op.
	for _, opt := range []string{defTerminalLetWindows, defTerminalClassic, defTerminalModern} {
		_ = applyTerminalSetting(opt)
	}
	if err := applyTerminalSetting("stale-value"); err != nil {
		t.Errorf("unknown terminal option should be ignored, got %v", err)
	}

	// The multi-file editor's huh theme hook resolves the live theme.
	e := newThemedMultiFileEditor("a;b")
	_ = e.HuhTheme()
}

// TestItemFromDefValueEdgeCases pins the value renderers itemFromDef builds
// for nil-valued, short-secret, path-titled, and custom fields.
func TestItemFromDefValueEdgeCases(t *testing.T) {
	shortSecret, pathVal, custom := "ab", "/tmp/some/where", "shown" //nolint:gosec // fake placeholder exercising the secret-masking path
	m := newTestModel(t, Options{ExtraSections: []config.Section[string]{{
		Title: "Edge",
		Fields: []config.FieldDef[string]{
			{Kind: config.FieldText, Title: "Nil Value"},
			{Kind: config.FieldText, Title: "Short Password", Value: &shortSecret},
			{Kind: config.FieldText, Title: "Data Path", Value: &pathVal},
			{Kind: config.FieldCustom, Title: "Plain Custom", Value: &custom},
		},
	}}})

	item := func(title string) settingItem { return m.items[findItemIndex(t, m, title)] }

	if got := item("Nil Value").value(); got != "" {
		t.Errorf("nil-valued field renders %q", got)
	}
	item("Nil Value").setValue("ignored") // nil guard: must not panic
	if got := item("Short Password").value(); got != "**" {
		t.Errorf("short secret = %q, want **", got)
	}
	if got := item("Data Path").value(); strings.Contains(got, "\\") {
		t.Errorf("path field not collapsed: %q", got)
	}
	if got := item("Plain Custom").value(); got != "shown" {
		t.Errorf("custom without display text = %q", got)
	}

	// huhFieldFromDef variants: select with height+validate, secret text with
	// validate, and the model-overlay kinds that build no huh field.
	sel := "a"
	f := m.huhFieldFromDef(config.FieldDef[string]{
		Kind: config.FieldSelect, Title: "S", Value: &sel, Height: 5,
		Options:  []huh.Option[string]{huh.NewOption("A", "a")},
		Validate: func(string) error { return nil },
	})
	if f == nil {
		t.Error("select def should build a field")
	}
	f = m.huhFieldFromDef(config.FieldDef[string]{
		Kind: config.FieldText, Title: "API Secret", Value: &sel,
		Validate: func(string) error { return nil },
	})
	if f == nil {
		t.Error("text def should build a field")
	}
	if m.huhFieldFromDef(config.FieldDef[string]{Kind: config.FieldDate}) != nil {
		t.Error("date def must not build a huh field")
	}
}

// TestNewLoadsCustomThemesDir: a themes directory beside the settings file is
// scanned at startup; unreadable files are logged, not fatal.
func TestNewLoadsCustomThemesDir(t *testing.T) {
	dir := t.TempDir()
	SetConfigDir(dir)
	t.Cleanup(func() { SetConfigDir("") })
	themes := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themes, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(themes, "broken.yaml"), []byte(":::not yaml"), 0o600); err != nil {
		t.Fatal(err)
	}

	m := NewWithOptions(Options{})
	if m == nil {
		t.Fatal("NewWithOptions returned nil")
	}
}

// TestKeyRecorderNavigationAndRecording covers the recorder's own state
// machine: wrap-around navigation, recording, replacement, and cancellation.
func TestKeyRecorderNavigationAndRecording(t *testing.T) {
	kr := NewKeyRecorder("a, b")
	if kr.Value() != "a,b" {
		t.Fatalf("Value() = %q, want a,b", kr.Value())
	}
	if kr.Init() != nil {
		t.Fatal("Init should be nil")
	}

	// Up from the first row wraps to the add slot; Down from there wraps home.
	_, _ = kr.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if kr.cursor != len(kr.keys) {
		t.Fatalf("cursor after wrap-up = %d", kr.cursor)
	}
	_, _ = kr.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if kr.cursor != 0 {
		t.Fatalf("cursor after wrap-down = %d", kr.cursor)
	}

	// Esc cancels an in-progress recording without touching the keys.
	_, _ = kr.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !kr.recording {
		t.Fatal("Enter should start recording")
	}
	_, _ = kr.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if kr.recording || len(kr.keys) != 2 {
		t.Fatalf("recording=%v keys=%v after cancel", kr.recording, kr.keys)
	}

	// Recording over an existing row replaces it in place.
	_, _ = kr.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_, _ = kr.Update(tea.KeyPressMsg{Code: 'z', Text: "z"})
	if kr.keys[0] != "z" {
		t.Fatalf("keys after replace = %v", kr.keys)
	}

	// The recording view renders both an existing-row and add-slot prompt.
	kr.recording = true
	if kr.View().Content == "" {
		t.Fatal("recording view should render")
	}
	kr.cursor = len(kr.keys)
	if kr.View().Content == "" {
		t.Fatal("add-slot recording view should render")
	}
	kr.recording = false

	// Esc outside recording aborts the whole editor; delete on the add slot
	// is a no-op; ctrl+s with a passing validation completes.
	_, _ = kr.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	if len(kr.keys) != 2 {
		t.Fatalf("delete on the add slot changed keys: %v", kr.keys)
	}
	_, _ = kr.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !kr.Aborted {
		t.Fatal("Esc should abort")
	}
	_, _ = kr.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if !kr.Done {
		t.Fatal("ctrl+s should complete without validation errors")
	}
}
