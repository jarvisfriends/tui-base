package settings

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jarvisfriends/tui-base/config"
)

// Local aliases keep goconst quiet about repeating the style literals the
// production defaults already use.
const (
	styleTopnav  = "topnav"
	styleSidebar = "sidebar"
)

// TestSettingsPersistenceRoundTrip: Save writes tui_settings.json under the
// configured directory, a fresh model loads it back, and ReloadFromDisk
// reports changed only when the file actually differs (FW-1's no-noise
// contract). Not parallel: SetConfigDir is package state.
func TestSettingsPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	SetConfigDir(dir)
	defer SetConfigDir("")

	m := NewWithOptions(Options{})
	if m.LoadedFromFile() {
		t.Fatal("fresh temp dir should have no settings file")
	}
	if !strings.HasPrefix(FilePath(), dir) {
		t.Fatalf("FilePath() = %q; want under %q", FilePath(), dir)
	}

	m.NavStyle = styleTopnav
	if err := m.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if !m.LoadedFromFile() {
		t.Fatal("Save should mark the model as persisted")
	}

	loaded := NewWithOptions(Options{})
	if !loaded.LoadedFromFile() || loaded.NavStyle != styleTopnav {
		t.Fatalf("reload: loaded=%v nav=%q", loaded.LoadedFromFile(), loaded.NavStyle)
	}

	// Nothing changed on disk since load — reload must be a quiet no-op.
	if changed, _ := loaded.ReloadFromDisk(); changed {
		t.Fatal("unchanged file reported as changed")
	}

	// An external edit (another model saving different values) must reload
	// with changed=true and a ThemeMsg command.
	m.NavStyle = styleSidebar
	m.ThemeMode = "light"
	if err := m.Save(); err != nil {
		t.Fatalf("save external edit: %v", err)
	}
	changed, cmd := loaded.ReloadFromDisk()
	if !changed || loaded.NavStyle != styleSidebar {
		t.Fatalf("external edit: changed=%v nav=%q", changed, loaded.NavStyle)
	}
	if cmd == nil {
		t.Fatal("changed reload should return the ThemeMsg command")
	}
	if msg, ok := cmd().(ThemeMsg); !ok || !msg.ApplyPreferences {
		t.Fatalf("reload cmd = %#v; want ThemeMsg with ApplyPreferences", cmd())
	}
}

// TestItemFromDefKinds drives itemFromDef/huhFieldFromDef through every
// FieldKind an ExtraSections app can supply: value rendering (select labels,
// secret masking, path collapsing, custom display text), setValue binding,
// and which kinds build a huh form versus a custom model versus nothing.
func TestItemFromDefKinds(t *testing.T) {
	t.Parallel()

	//nolint:gosec // fake placeholder exercising the secret-masking path
	sel, txt, secret, file, multi, custom := "b", "plain", "not-a-real-value", `C:\data\f.txt`, "a;b", "x"
	m := NewWithOptions(Options{ExtraSections: []config.Section[string]{{
		Title: "App",
		Fields: []config.FieldDef[string]{
			{Kind: config.FieldSelect, Title: "Choice", Value: &sel, Options: []huh.Option[string]{
				huh.NewOption("Label A", "a"), huh.NewOption("Label B", "b"),
			}},
			{Kind: config.FieldText, Title: "Name", Value: &txt},
			{Kind: config.FieldText, Title: "API Token", Value: &secret},
			{Kind: config.FieldFilePicker, Title: "Input File", Value: &file},
			{Kind: config.FieldMultiFilePicker, Title: "Sources", Value: &multi},
			{
				Kind: config.FieldCustom, Title: "Special", Value: &custom,
				CustomFieldText: "press enter", CustomModelBuilder: func() tea.Model { return nil },
			},
		},
	}}})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	item := func(title string) settingItem {
		t.Helper()
		idx := findItemIndex(t, m, title)
		return m.items[idx]
	}

	if got := item("Choice").value(); got != "Label B" {
		t.Errorf("select value shows %q; want its label", got)
	}
	if got := item("Name").value(); got != "plain" {
		t.Errorf("text value = %q", got)
	}
	if got := item("API Token").value(); !strings.HasPrefix(got, "****") ||
		strings.Contains(got, "not-a-real") {
		t.Errorf("secret not masked: %q", got)
	}
	if got := item("Special").value(); got != "press enter" {
		t.Errorf("custom display text = %q", got)
	}

	// setValue writes through to the app's bound variable.
	item("Name").setValue("renamed")
	if txt != "renamed" {
		t.Errorf("setValue did not write through: %q", txt)
	}

	// Form-backed kinds build a form; model-backed and custom kinds don't.
	for _, tc := range []struct {
		title    string
		wantForm bool
	}{
		{"Choice", true},
		{"Name", true},
		{"Input File", true},
		{"Sources", false},
		{"Special", false},
	} {
		if got := item(tc.title).buildForm() != nil; got != tc.wantForm {
			t.Errorf("%s: buildForm != nil = %v; want %v", tc.title, got, tc.wantForm)
		}
	}
	if item("Sources").buildModel() == nil {
		t.Error("multi-file field should build a custom model")
	}
}

// TestSettingsEditingLifecycle: Enter opens the editor (View switches to the
// overlay and CapturesKeys reports true), Esc aborts back to the overview.
func TestSettingsEditingLifecycle(t *testing.T) {
	t.Parallel()

	m := NewWithOptions(Options{})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	if cmd := m.Init(); cmd != nil {
		t.Fatal("Init should be a no-op")
	}
	if m.CapturesKeys() {
		t.Fatal("overview must not capture keys")
	}

	selectItemForTest(m, findItemIndex(t, m, "Navigation Style"))
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.editOverlay.IsOpen() {
		t.Fatal("Enter should open the edit overlay")
	}
	if !m.CapturesKeys() {
		t.Fatal("an open editor must capture keys")
	}
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "Navigation Style") {
		t.Fatalf("editing view missing form:\n%s", v)
	}

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.editOverlay.IsOpen() {
		t.Fatal("Esc should abort the edit")
	}
}
