// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package router

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/jarvisfriends/inspector"
	"github.com/jarvisfriends/snap/navigation"
	"github.com/jarvisfriends/snap/notifications"
	"github.com/jarvisfriends/snap/status"
	"github.com/jarvisfriends/snap/styles"
	"github.com/jarvisfriends/tui-base/filewatch"
	"github.com/jarvisfriends/tui-base/pages/settings"
)

// navStyleSidebar mirrors the production tabs/topnav literals for the one
// style constant router.go does not define.
const navStyleSidebar = "sidebar"

// newSizedRouter builds a router with an isolated config dir and a real size.
func newSizedRouter(t *testing.T, opts Options) *RouterModel {
	t.Helper()
	if opts.ConfigDir == "" {
		opts.ConfigDir = t.TempDir()
	}
	m := NewWithOptions(opts)
	t.Cleanup(m.Close)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	return m
}

// TestWindowsTerminalPathsOffWindows: the fragment installers refuse to run
// off Windows, and the relaunch gate is a silent no-op. On Windows these are
// the real installers — they would write a fragment into the user's Windows
// Terminal settings — so the assertions only hold (and only run) elsewhere.
func TestWindowsTerminalPathsOffWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("installers are functional on Windows; off-Windows refusal is what this pins")
	}
	if _, err := InstallWindowsTerminalProfile(WindowsTerminalProfile{AppName: "X"}); err == nil {
		t.Error("InstallWindowsTerminalProfile should fail off Windows")
	}
	if err := UninstallWindowsTerminalProfile("X"); err == nil {
		t.Error("UninstallWindowsTerminalProfile should fail off Windows")
	}
	if relaunched, err := MaybeRelaunchInWindowsTerminal(TerminalRelaunchConfig{}); relaunched || err != nil {
		t.Errorf("MaybeRelaunchInWindowsTerminal = (%v, %v); want (false, nil)", relaunched, err)
	}
}

// TestForcedColorProfileAllAliases pins every accepted env value and the
// detection fallback for the internal env-var variant.
func TestForcedColorProfileAllAliases(t *testing.T) {
	for val, want := range map[string]colorprofile.Profile{
		"ansi256": colorprofile.ANSI256,
		"256":     colorprofile.ANSI256,
		"ansi":    colorprofile.ANSI,
		"16":      colorprofile.ANSI,
		"ascii":   colorprofile.Ascii,
		"none":    colorprofile.Ascii,
		"notty":   colorprofile.NoTTY,
	} {
		t.Setenv(ColorProfileEnvVar, val)
		got, ok := ForcedColorProfile()
		if !ok || got != want {
			t.Errorf("%q: got %v ok=%v; want %v", val, got, ok, want)
		}
	}

	// Unset: EffectiveColorProfile falls back to detection for both variants.
	t.Setenv(ColorProfileEnvVar, "")
	_ = EffectiveColorProfile()
	_ = effectiveColorProfileForEnvVar("COVER_APP_COLOR_PROFILE")
}

func TestRouterEnvVarAccessors(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "My App"})
	if got := m.ColorProfileEnvVar(); got != "MY_APP_COLOR_PROFILE" {
		t.Errorf("ColorProfileEnvVar() = %q", got)
	}
	if got := m.DebugEnvVar(); got != "MY_APP_DEBUG" {
		t.Errorf("DebugEnvVar() = %q", got)
	}
	// Names that reduce to nothing fall back to the framework prefix.
	if got := screamingSnake("!!!"); got != "TUI_BASE" {
		t.Errorf("screamingSnake(\"!!!\") = %q", got)
	}
}

// coverProvider is a minimal inspector tab for the extension hooks.
type coverProvider struct{ started, stopped bool }

func (p *coverProvider) TabName() string                     { return "Cover" }
func (p *coverProvider) BuildRows(*styles.AppStyle) []string { return []string{"row"} }
func (p *coverProvider) RefreshInterval() time.Duration      { return 0 }
func (p *coverProvider) Start()                              { p.started = true }
func (p *coverProvider) Stop()                               { p.stopped = true }

func TestInspectorTabAndStatusSegmentHooks(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Hooks"})

	p := &coverProvider{}
	m.RegisterInspectorTab(p)
	m.RemoveInspectorTab("Cover")
	if !p.stopped {
		t.Error("RemoveInspectorTab should stop the provider")
	}

	m.SetStatusSegment("git", func() string { return "main" })
	m.SetStatusSegment("git", nil)
}

func TestBackgroundColorMsgAutoSwitchesMode(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "BG"})

	// A light terminal background flips the theme to light mode.
	_, cmd := m.Update(tea.BackgroundColorMsg{Color: lightColor{}})
	if cmd == nil {
		t.Fatal("BackgroundColorMsg should schedule resize + color sync")
	}
	// The same report again matches the current mode: the quiet branch.
	_, _ = m.Update(tea.BackgroundColorMsg{Color: lightColor{}})
	// And a dark background flips back.
	_, _ = m.Update(tea.BackgroundColorMsg{Color: darkColor{}})
}

type lightColor struct{}

func (lightColor) RGBA() (r, g, b, a uint32) { return 0xffff, 0xffff, 0xffff, 0xffff }

type darkColor struct{}

func (darkColor) RGBA() (r, g, b, a uint32) { return 0, 0, 0, 0xffff }

// TestApplyThemeMsgAndSettle: the inspector's host-agnostic theme message is
// translated to the settings dialect, and only the newest settle generation
// runs the relayout.
func TestApplyThemeMsgAndSettle(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Theme"})

	if _, cmd := m.Update(inspector.ApplyThemeMsg{ID: m.settingsPage.ColorThemeID}); cmd == nil {
		t.Fatal("ApplyThemeMsg should schedule the debounced settle")
	}

	// A stale settle (older generation) is dropped.
	if _, cmd := m.Update(themeSettleMsg{gen: m.themePreviewGen - 1}); cmd != nil {
		t.Error("stale themeSettleMsg should be ignored")
	}
	// The current generation performs the relayout + terminal sync.
	if _, cmd := m.Update(themeSettleMsg{gen: m.themePreviewGen}); cmd == nil {
		t.Error("current themeSettleMsg should relayout")
	}

	// The explicit terminal color sync message and its raw OSC command.
	_, cmd := m.Update(syncTerminalColorsMsg{})
	if cmd == nil {
		t.Error("syncTerminalColorsMsg should emit the OSC command")
	}
}

// TestThemeMsgDebugLoggingPath exercises the verbose-debug branches of the
// theme handler and handleResizeCmd, plus the nil-colors adoption path.
func TestThemeMsgDebugLoggingPath(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Debug Theme"})
	t.Setenv(m.DebugEnvVar(), "1")

	m.colors = nil // adoption branch: the router takes the new palette pointer
	_, cmd := m.Update(settings.ThemeMsg{
		ID:               m.settingsPage.ColorThemeID,
		Mode:             m.settingsPage.ThemeMode,
		Style:            m.settingsPage.StylePreset,
		ApplyPreferences: true,
	})
	if cmd == nil {
		t.Fatal("ThemeMsg should schedule the settle tick")
	}
	if m.colors == nil {
		t.Fatal("router did not adopt the new palette")
	}
	// Run the settle at the current generation with debug logging active.
	_, _ = m.Update(themeSettleMsg{gen: m.themePreviewGen})
}

func TestNotificationsSettingsMsgTogglesPersistence(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Notif"})
	if m.notifPersistPath == "" {
		t.Fatal("expected a default persist path with a config dir")
	}

	_, _ = m.Update(settings.NotificationsSettingsMsg{Enabled: true, Persist: true})
	_, _ = m.Update(settings.NotificationsSettingsMsg{Enabled: true, Persist: false})
}

func TestKeybindingsChangedMsgReappliesKeys(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Keys"})
	_, _ = m.Update(settings.KeybindingsChangedMsg{
		CustomKeys: map[string]string{"Quit": "ctrl+q"},
	})
	if !strings.Contains(strings.Join(m.keys.Quit.Keys(), ","), "ctrl+q") {
		t.Error("custom quit binding not applied")
	}
}

func TestGatesChangedMessages(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Gates"})

	_, _ = m.Update(settings.GatesChangedMsg{}) // must not panic; relayouts

	// The inspector-originated flip re-broadcasts the settings contract.
	_, cmd := m.Update(inspector.GatesChangedMsg{Values: map[string]bool{"x": true}})
	if cmd == nil {
		t.Fatal("inspector.GatesChangedMsg should re-broadcast")
	}
	found := false
	collectMsgs(cmd, func(msg tea.Msg) {
		if _, ok := msg.(settings.GatesChangedMsg); ok {
			found = true
		}
	})
	if !found {
		t.Error("settings.GatesChangedMsg was not re-broadcast")
	}
}

// collectMsgs runs a command tree and hands every produced message to fn
// without feeding anything back into a model.
func collectMsgs(cmd tea.Cmd, fn func(tea.Msg)) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			collectMsgs(c, fn)
		}
		return
	}
	if msg != nil {
		fn(msg)
	}
}

func TestInfoModalMessages(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Info", AppVersion: "9.9.9"})

	m.infoModal.Toggle(100, 32)
	_, _ = m.Update(status.InfoModalScrollMsg{Up: false})
	_, _ = m.Update(status.InfoModalScrollMsg{Up: true})
	_, _ = m.Update(status.CloseInfoModalMsg{})
	if m.infoModal.IsVisible() {
		t.Error("info modal should be closed")
	}
}

func TestClickRegionMsgRouting(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Regions"})

	_, _ = m.Update(status.ClickRegionMsg{Name: status.NotificationsRegionName})
	if !m.status.IsHistoryVisible() {
		t.Error("notifications region click should open the panel")
	}
	_, _ = m.Update(status.ClickRegionMsg{Name: status.InfoRegionName})
	if !m.infoModal.IsVisible() {
		t.Error("info modal should have opened")
	}
	_, _ = m.Update(status.ClickRegionMsg{Name: "settings"})
	if got := m.nav.GetPages()[m.nav.GetActiveIndex()].ID; got != "settings" {
		t.Errorf("page region click landed on %q", got)
	}
	if _, cmd := m.Update(status.ClickRegionMsg{Name: "no-such-region"}); cmd != nil {
		t.Error("unknown region click should be a no-op")
	}
}

func TestNavFocusAndCollapseMessages(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Focus"})
	_, _ = m.Update(settings.NavStyleMsg{Style: navStyleSidebar})

	_, _ = m.Update(navigation.NavFocusMsg{Focused: true})
	if !m.sidebarFocused {
		t.Error("NavFocusMsg did not update focus state")
	}
	_, _ = m.Update(navigation.CollapseToggleMsg{})
}

func TestReplaceAppPagesMsgAndRegisterPage(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Replace"})

	const secondTitle = "Second Page"
	pages := []RegisteredPage{
		{Title: "First Page", Model: nullModel{}},
		{Title: secondTitle, Model: nullModel{}},
		{Title: "", Model: nullModel{}}, // skipped: no title
	}
	_, _ = m.Update(ReplaceAppPagesMsg{Pages: pages, ActiveTitle: secondTitle, ActiveIndex: -1})
	if got := m.nav.GetPages()[m.nav.GetActiveIndex()].Title; got != secondTitle {
		t.Fatalf("active page after replace = %q; want %q", got, secondTitle)
	}

	if cmd := m.RegisterPage("Gamma", nullModel{}); cmd != nil {
		t.Error("nullModel Init should be nil")
	}
	found := false
	for _, p := range m.nav.GetPages() {
		if p.Title == "Gamma" {
			found = true
		}
	}
	if !found {
		t.Error("RegisterPage did not add the page")
	}

	// Selecting the already-active page is a hook no-op.
	if cmd := m.switchActivePage(m.activePageIndex()); cmd != nil {
		t.Error("re-selecting the active page should be a no-op")
	}
}

func TestKonamiSequenceFiresEasterEgg(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Konami"})

	// A wrong key that is also the sequence start restarts progress.
	m.konamiProgress = 2
	if cmd := m.advanceKonami(konamiKeyUp); cmd != nil {
		t.Fatal("restart key must not fire the egg")
	}
	if m.konamiProgress != 1 {
		t.Fatalf("konamiProgress = %d; want 1 after restart", m.konamiProgress)
	}
	// A completely wrong key resets progress.
	_ = m.advanceKonami("x")
	if m.konamiProgress != 0 {
		t.Fatalf("konamiProgress = %d; want 0 after mismatch", m.konamiProgress)
	}

	// The full sequence, via the router's key handler, fires the notification.
	seq := []tea.KeyPressMsg{
		{Code: tea.KeyUp},
		{Code: tea.KeyUp},
		{Code: tea.KeyDown},
		{Code: tea.KeyDown},
		{Code: tea.KeyLeft},
		{Code: tea.KeyRight},
		{Code: tea.KeyLeft},
		{Code: tea.KeyRight},
		{Code: 'b', Text: "b"},
		{Code: 'a', Text: "a"},
	}
	var lastCmd tea.Cmd
	for _, k := range seq {
		_, lastCmd = m.Update(k)
	}
	if lastCmd == nil {
		t.Fatal("completing the Konami sequence should return a command")
	}
	if _, ok := lastCmd().(notifications.AddMsg); !ok {
		t.Fatalf("expected the easter-egg notification, got %#v", lastCmd())
	}
}

func TestToggleNavKeyWalksThreeStates(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Nav Toggle"})
	_, _ = m.Update(settings.NavStyleMsg{Style: navStyleSidebar})

	ctrlB := tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl}

	// Visible + unfocused → focused.
	_, _ = m.Update(ctrlB)
	if !m.navigationVisible || !m.sidebarFocused {
		t.Fatalf("state after 1st toggle = visible %v focused %v", m.navigationVisible, m.sidebarFocused)
	}
	// Visible + focused → hidden.
	_, _ = m.Update(ctrlB)
	if m.navigationVisible || m.sidebarFocused {
		t.Fatalf("state after 2nd toggle = visible %v focused %v", m.navigationVisible, m.sidebarFocused)
	}
	// Hidden → visible (unfocused).
	_, _ = m.Update(ctrlB)
	if !m.navigationVisible || m.sidebarFocused {
		t.Fatalf("state after 3rd toggle = visible %v focused %v", m.navigationVisible, m.sidebarFocused)
	}

	// Full-help and status toggles ride the same always-active switch.
	_, _ = m.Update(tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl})
	_, _ = m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	// ctrl+g jumps to settings.
	_, _ = m.Update(tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl})
	if got := m.nav.GetPages()[m.nav.GetActiveIndex()].ID; got != navigation.PageIDSettings {
		t.Fatalf("ctrl+g landed on %q", got)
	}
}

func TestNumberKeySelectOnTopNav(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Digits"})
	_, _ = m.Update(settings.NavStyleMsg{Style: navStyleTopnav})
	_, _ = m.Update(settings.NavShowNumbersMsg{Show: true})
	_, _ = m.Update(settings.NavNumberSelectMsg{Enabled: true})
	if !m.navNumberSelect {
		t.Fatal("NavNumberSelectMsg did not enable the shortcut")
	}

	// "2" jumps to the second page (Settings on the default page set).
	_, _ = m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	if got := m.nav.GetActiveIndex(); got != 1 {
		t.Fatalf("active index after '2' = %d; want 1", got)
	}
	// A digit beyond the page count falls through unhandled.
	_, _ = m.Update(tea.KeyPressMsg{Code: '9', Text: "9"})
	if got := m.nav.GetActiveIndex(); got != 1 {
		t.Fatalf("out-of-range digit moved the page to %d", got)
	}

	// navDigitIndex parses digits only.
	if i, ok := navDigitIndex(tea.KeyPressMsg{Code: '3', Text: "3"}); !ok || i != 2 {
		t.Errorf("navDigitIndex(3) = (%d, %v)", i, ok)
	}
	if _, ok := navDigitIndex(tea.KeyPressMsg{Code: 'a', Text: "a"}); ok {
		t.Error("navDigitIndex must reject non-digits")
	}

	// Direct out-of-range jumps are refused.
	if cmd := m.cyclePageTo(-1); cmd != nil {
		t.Error("cyclePageTo(-1) should be nil")
	}
	if cmd := m.cyclePageTo(99); cmd != nil {
		t.Error("cyclePageTo(99) should be nil")
	}
	// Shift+Tab cycles backwards on a top-docked nav.
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
}

func TestSidebarFocusKeyTransitions(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Sidebar Focus"})
	_, _ = m.Update(settings.NavStyleMsg{Style: navStyleSidebar})

	// Esc / Left focuses the sidebar from the page.
	if _, handled := m.handleSidebarNavKey(tea.KeyPressMsg{Code: tea.KeyEscape}); !handled {
		t.Fatal("Esc should move focus to the sidebar")
	}
	if !m.sidebarFocused {
		t.Fatal("sidebar not focused after Esc")
	}
	// Right / Enter / Tab returns focus to the page.
	if _, handled := m.handleSidebarNavKey(tea.KeyPressMsg{Code: tea.KeyEnter}); !handled {
		t.Fatal("Enter should move focus back to the page")
	}
	if m.sidebarFocused {
		t.Fatal("sidebar still focused after Enter")
	}
	// Tab while the page is focused returns to the sidebar (next-region half).
	if _, handled := m.handleSidebarNavKey(tea.KeyPressMsg{Code: 'p', Text: "p"}); handled {
		t.Fatal("a plain letter should fall through")
	}
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if !m.sidebarFocused {
		t.Fatal("Tab should return focus to the sidebar")
	}
	// Up/Down while focused fall through to the sidebar itself.
	if _, handled := m.handleSidebarNavKey(tea.KeyPressMsg{Code: tea.KeyUp}); handled {
		t.Fatal("Up must fall through to the sidebar component")
	}
	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})

	if !sidebarNavConsumesKey(tea.KeyPressMsg{Code: tea.KeyEnter}) {
		t.Error("Enter is a sidebar key")
	}
	if sidebarNavConsumesKey(tea.KeyPressMsg{Code: 'z', Text: "z"}) {
		t.Error("plain letters are not sidebar keys")
	}
}

// TestNilNavFallbacks covers the defensive nil-navigator branches.
func TestNilNavFallbacks(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Nil Nav"})
	m.nav = nil

	if got := m.activeWindowTitle(); got != "Nil Nav" {
		t.Errorf("activeWindowTitle = %q", got)
	}
	if _, ok := m.activatePageByID("settings"); ok {
		t.Error("activatePageByID should fail without a nav")
	}
	if cmd := m.switchActivePage(0); cmd != nil {
		t.Error("switchActivePage without nav should be nil")
	}
	if cmd := m.cyclePage(1); cmd != nil {
		t.Error("cyclePage without nav should be nil")
	}
	if cmd := m.cyclePageTo(0); cmd != nil {
		t.Error("cyclePageTo without nav should be nil")
	}
	if !m.pageMatchesTarget(0, "anything") {
		t.Error("pageMatchesTarget without nav should broadcast")
	}
	if got := m.navReservedWidth(); got != 0 {
		t.Errorf("navReservedWidth without nav = %d", got)
	}
	if got := m.activePageIndex(); got != 0 {
		t.Errorf("activePageIndex without nav = %d", got)
	}
}

func TestPageMatchesTargetOutOfRange(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Target"})
	if m.pageMatchesTarget(99, "settings") {
		t.Error("out-of-range page index must not match")
	}
	if !m.pageMatchesTarget(0, m.nav.GetPages()[0].ID) {
		t.Error("in-range ID should match")
	}
}

// TestNotificationAddSchedulesExpiry: notification traffic routed through the
// manager returns the TTL expiry command.
func TestNotificationAddSchedulesExpiry(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Expiry"})
	_, cmd := m.Update(notifications.AddMsg{
		Content:  "with ttl",
		Severity: notifications.SeverityInfo,
		TTL:      time.Minute,
	})
	if cmd == nil {
		t.Fatal("AddMsg with a TTL should produce commands")
	}
}

// TestKeyReleaseWhileModalOpen covers the non-press key-message modality gate.
func TestKeyReleaseWhileModalOpen(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Release"})
	m.inspector.ToggleVisible()
	_, _ = m.Update(tea.KeyReleaseMsg{Code: 'x', Text: "x"})
}

// TestStartupPathsFromPersistedSettings drives NewWithOptions through the
// persisted-settings branches: topnav style, nav numbers, notification
// persistence, and a failing log-init destination.
func TestStartupPathsFromPersistedSettings(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "cfg")
	if err := os.MkdirAll(cfg, 0o750); err != nil {
		t.Fatal(err)
	}
	persisted := `{
		"nav_style": "topnav",
		"nav_show_numbers": true,
		"notifications_enabled": true,
		"notifications_persist": true,
		"log_output": "dir",
		"log_path": ` + strconv.Quote(filepath.Join(blocker, "logs")) + `
	}`
	if err := os.WriteFile(filepath.Join(cfg, "tui_settings.json"), []byte(persisted), 0o600); err != nil {
		t.Fatal(err)
	}

	m := newSizedRouter(t, Options{AppName: "Persisted", ConfigDir: cfg})
	if _, ok := m.nav.(*navigation.MinimalTopNav); !ok {
		t.Errorf("nav = %T; want the minimal top nav", m.nav)
	}
	if !m.navShowNumbers {
		t.Error("nav numbers preference not adopted")
	}

	// Tabs style from disk too.
	cfg2 := filepath.Join(dir, "cfg2")
	if err := os.MkdirAll(cfg2, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg2, "tui_settings.json"), []byte(`{"nav_style":"tabs"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = newSizedRouter(t, Options{AppName: "Persisted Tabs", ConfigDir: cfg2})
}

// TestSettingsWatchLifecycle: the watcher starts with the option, survives a
// no-op event, dies on ErrorMsg, and Close is idempotent. A config dir that
// cannot be created disables watching instead of failing startup.
func TestSettingsWatchLifecycle(t *testing.T) {
	m := newSizedRouter(t, Options{AppName: "Watch", WatchSettingsFile: true})
	if m.settingsWatcher == nil {
		t.Fatal("watcher should be armed with WatchSettingsFile")
	}
	if m.settingsWatchInit() == nil {
		t.Fatal("settingsWatchInit should return the arm command")
	}

	_, _ = m.Update(filewatch.ErrorMsg{Path: settings.FilePath(), Err: os.ErrClosed})
	if m.settingsWatcher != nil {
		t.Fatal("watcher should be released after ErrorMsg")
	}
	if m.settingsWatchInit() != nil {
		t.Fatal("settingsWatchInit should be nil once stopped")
	}
	m.Close() // second stop: the nil fast path

	// Un-creatable settings dir: watching is disabled, startup succeeds.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	m2 := newSizedRouter(t, Options{
		AppName:           "Watch Broken",
		WatchSettingsFile: true,
		ConfigDir:         filepath.Join(blocker, "sub"),
	})
	if m2.settingsWatcher != nil {
		t.Fatal("watcher should be disabled when the settings dir is unusable")
	}
}
