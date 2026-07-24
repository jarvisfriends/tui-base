package home

import (
	"testing"

	"github.com/jarvisfriends/snap/styles"

	tea "charm.land/bubbletea/v2"
)

// key delivers a single printable key press through Update.
func pressKey(m *HomePageModel, r rune) tea.Cmd {
	_, cmd := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	return cmd
}

// TestHomeShowcaseKeys covers the keyboard showcase handlers added to the home
// page Update loop: t (theme), o (pill shape), p (progress bar style),
// r (sparkline style), and f (effects tier). Each key mutates model state or
// returns a command; this pins the t/o/p/r/f switch arms so they stay covered.
func TestHomeShowcaseKeys(t *testing.T) {
	styles.VerifyRegistry() // 't' cycles tints; ensure the registry is initialized
	m := sized(t, 100, 40)

	// 'o' cycles the pill shape index, wrapping at len(PillShapes()).
	if n := len(styles.PillShapes()); n > 1 {
		shape0 := m.shape
		pressKey(m, 'o')
		if m.shape == shape0 {
			t.Errorf("'o' should advance pill shape: still %d", m.shape)
		}
	}

	// 'r' advances the sparkline style within sparklineStyleCount.
	spark0 := m.sparkStyle
	pressKey(m, 'r')
	if m.sparkStyle == spark0 {
		t.Errorf("'r' should advance sparkline style: still %v", m.sparkStyle)
	}

	// 'f' cycles the effects tier (Minimal → Medium → High → …).
	fx0 := m.Effects
	pressKey(m, 'f')
	if m.Effects == fx0 {
		t.Errorf("'f' should cycle effects tier: still %v", m.Effects)
	}

	// 'p' cycles the progress-bar style (possibly returning an anim command).
	prog0 := m.progStyle
	pressKey(m, 'p')
	if m.progStyle == prog0 {
		t.Errorf("'p' should cycle progress style: still %v", m.progStyle)
	}

	// 't' cycles the theme; with tints registered it returns a ThemeMsg command.
	if cmd := pressKey(m, 't'); cmd == nil {
		t.Error("'t' should return a theme-cycle command when tints are registered")
	}
}

// TestHomeShowcaseKeysWrap drives each cycling key enough times to wrap past its
// modulus, covering the wrap-around arithmetic in the o/r/f/p handlers.
func TestHomeShowcaseKeysWrap(t *testing.T) {
	styles.VerifyRegistry()
	m := sized(t, 100, 40)
	for range len(styles.PillShapes()) + 1 {
		pressKey(m, 'o')
	}
	for range sparklineStyleCount + 1 {
		pressKey(m, 'r')
	}
	for range 4 { // Effects has three tiers; 4 presses wraps at least once.
		pressKey(m, 'f')
		pressKey(m, 'p')
	}
	// A View render after all the mutations must still succeed (no panic / empty).
	if m.View().Content == "" {
		t.Fatal("home View content empty after cycling showcase keys")
	}
}
