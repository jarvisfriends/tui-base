package router

import (
	"testing"

	"github.com/jarvisfriends/tui-base/navigation"
	"github.com/jarvisfriends/tui-base/notifications"
	"github.com/jarvisfriends/tui-base/pages/settings"
	"github.com/jarvisfriends/tui-base/testutil"

	tea "charm.land/bubbletea/v2"
)

// TestConformanceSuite runs the shared tui-base conformance checks against the
// router itself. Derived apps (dash, plex-maint, media/tui) run the same
// testutil.* helpers against their own RouterModel — this both guards the
// framework and serves as the reference for how to invoke them.
func TestConformanceSuite(t *testing.T) {
	t.Run("FitsViewport", func(t *testing.T) {
		testutil.CheckFitsViewport(t, New())
	})

	t.Run("ThemeResponsive", func(t *testing.T) {
		testutil.CheckThemeResponsive(t, New(),
			settings.ThemeMsg{ID: "dracula", Mode: "dark", ApplyPreferences: true},
			settings.ThemeMsg{ID: "dracula", Mode: "light", ApplyPreferences: true},
		)
	})

	t.Run("StatusBarVisibleEverywhere", func(t *testing.T) {
		m := New()
		var states []tea.Msg
		// Visit every page.
		for i := range m.nav.GetPages() {
			states = append(states, navigation.SelectedMsg{PageIndex: i})
		}
		// Open the inspector overlay (Ctrl+D) and the settings page (Ctrl+,);
		// the status bar must remain visible with overlays/prompts on screen.
		states = append(states,
			tea.KeyPressMsg{Text: "ctrl+d"},
			tea.KeyPressMsg{Text: "ctrl+,"},
		)
		testutil.CheckStatusBarVisible(t, m, states)
	})
}

// TestKonamiSecretFiresNotification verifies the hidden key sequence emits a
// notification (the placeholder "secret menu") and that intermediate keys are
// observed without being consumed.
func TestKonamiSecretFiresNotification(t *testing.T) {
	m := New()
	seq := []string{"up", "up", "down", "down", "left", "right", "left", "right", "b"}
	for _, k := range seq {
		_, cmd := m.Update(tea.KeyPressMsg{Text: k})
		// Intermediate keys must not fire the easter egg.
		if cmd != nil {
			if _, ok := cmd().(notifications.AddMsg); ok {
				t.Fatalf("sequence fired early at key %q", k)
			}
		}
	}
	// The final "a" completes the sequence.
	_, cmd := m.Update(tea.KeyPressMsg{Text: "a"})
	if cmd == nil {
		t.Fatal("completing the Konami sequence should return a command")
	}
	if _, ok := cmd().(notifications.AddMsg); !ok {
		t.Fatalf("expected notifications.AddMsg, got %T", cmd())
	}
}
