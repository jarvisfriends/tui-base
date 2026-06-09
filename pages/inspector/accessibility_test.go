package inspector

import (
	"strings"
	"testing"

	"github.com/jarvisfriends/tui-base/pages/settings"
	"github.com/jarvisfriends/tui-base/theme"

	tea "charm.land/bubbletea/v2"
	tint "github.com/lrstanley/bubbletint/v2"
)

func init() {
	tint.NewDefaultRegistry()
}

func TestAccessibilityPanel_LifecycleAndKeys(t *testing.T) {
	p := NewAccessibilityPanel()
	if p.IsVisible() {
		t.Error("AccessibilityPanel should not be visible initially")
	}

	_ = theme.SetCurrentTint("dracula")
	p.SetColors(theme.Active())
	p.SetSize(80, 20)

	if cmd := p.Init(); cmd != nil {
		t.Error("expected Init to return nil")
	}

	// Toggle on
	p.Toggle()
	if !p.IsVisible() {
		t.Fatal("expected visible after Toggle")
	}

	// Re-toggle should close
	p.Toggle()
	if p.IsVisible() {
		t.Error("expected hidden after second Toggle")
	}
	p.Toggle() // keep it open

	// 1. Key "down" and "j" (move cursor)
	p.cursor = 0
	_, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if p.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", p.cursor)
	}
	_, _ = p.Update(tea.KeyPressMsg{Text: "j"})
	if p.cursor != 2 {
		t.Errorf("expected cursor 2, got %d", p.cursor)
	}

	// 2. Key "up" and "k"
	_, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if p.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", p.cursor)
	}
	_, _ = p.Update(tea.KeyPressMsg{Text: "k"})
	if p.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", p.cursor)
	}

	// 3. Toggle CVD inclusion filters ("1", "2", "3")
	if p.cvd[0] || p.cvd[1] || p.cvd[2] {
		t.Error("expected CVD inclusion filters false initially")
	}
	_, _ = p.Update(tea.KeyPressMsg{Text: "1"})
	if !p.cvd[0] {
		t.Error("expected CVD Protanopia filter enabled after '1'")
	}
	_, _ = p.Update(tea.KeyPressMsg{Text: "2"})
	if !p.cvd[1] {
		t.Error("expected CVD Deuteranopia filter enabled after '2'")
	}
	_, _ = p.Update(tea.KeyPressMsg{Text: "3"})
	if !p.cvd[2] {
		t.Error("expected CVD Tritanopia filter enabled after '3'")
	}

	// 4. Toggle dark/light ("d", "l")
	if !p.showDark || !p.showLight {
		t.Error("expected dark/light shown by default")
	}
	_, _ = p.Update(tea.KeyPressMsg{Text: "d"})
	if p.showDark {
		t.Error("expected showDark false after 'd'")
	}
	_, _ = p.Update(tea.KeyPressMsg{Text: "d"}) // toggle back on
	if !p.showDark {
		t.Error("expected showDark true after toggling back")
	}

	_, _ = p.Update(tea.KeyPressMsg{Text: "l"})
	if p.showLight {
		t.Error("expected showLight false after 'l'")
	}
	_, _ = p.Update(tea.KeyPressMsg{Text: "l"}) // toggle back on
	if !p.showLight {
		t.Error("expected showLight true after toggling back")
	}

	// 5. Enter should trigger theme change message
	p.cursor = 0
	if len(p.view) > 0 {
		_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if cmd == nil {
			t.Fatal("expected cmd on Enter")
		}
		msg := cmd()
		if _, ok := msg.(settings.ThemeMsg); !ok {
			t.Errorf("expected ThemeMsg on Enter, got %T", msg)
		}
	}

	// 6. View rendering
	v := p.View()
	if v.Content == "" {
		t.Error("expected non-empty View content")
	}

	// Test rendering with specific cursor highlights and issue lists
	p.cursor = 0
	vSelected := p.View()
	if !strings.Contains(vSelected.Content, "▶") {
		t.Error("expected selected cursor pointer in view")
	}
}

func TestAccessibilityPanel_NonVisibleIgnoresKeys(t *testing.T) {
	p := NewAccessibilityPanel()
	p.Toggle()
	p.Toggle() // visible = false

	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Error("expected nil cmd for non-visible panel keys")
	}
	v := p.View()
	if v.Content != "" {
		t.Errorf("expected empty view for non-visible panel, got %q", v.Content)
	}

	// Ignore non-keypress msg
	_, cmd2 := p.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	if cmd2 != nil {
		t.Error("expected nil cmd for non-keypress msg")
	}
}
