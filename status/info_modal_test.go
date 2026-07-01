package status

import (
	"os"
	"strings"
	"testing"

	"github.com/jarvisfriends/tui-base/keys"
	"github.com/jarvisfriends/tui-base/theme"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"
)

func TestMain(m *testing.M) {
	tint.NewDefaultRegistry()
	os.Exit(m.Run())
}

func TestInfoModal_Lifecycle(t *testing.T) {
	im := NewInfoModal()
	if im.IsVisible() {
		t.Error("InfoModal should not be visible initially")
	}

	im.SetAppName("TestApp")
	im.SetVersion("v1.2.3")
	keyMap := keys.DefaultKeyMap()
	im.SetKeys(keyMap)

	if cmd := im.Init(); cmd != nil {
		t.Error("expected Init to return nil")
	}
	if im.Name() != "InfoModal" {
		t.Errorf("expected name 'InfoModal', got %q", im.Name())
	}

	// 1. Open
	im.Open(80, 24)
	if !im.IsVisible() {
		t.Fatal("expected visible after Open")
	}

	// 2. View
	_ = theme.SetCurrentTint("dracula")
	v := im.View()
	if v.Content == "" {
		t.Fatal("expected non-empty view content when open")
	}
	if !strings.Contains(v.Content, "TestApp") || !strings.Contains(v.Content, "v1.2.3") {
		t.Errorf("view content missing app/version info: %s", v.Content)
	}

	// 3. Bounds
	bx, by, bw, bh := im.Bounds()
	if bw <= 0 || bh <= 0 || bx < 0 || by < 0 {
		t.Errorf("invalid bounds: x=%d y=%d w=%d h=%d", bx, by, bw, bh)
	}

	// 4. Close
	im.Close()
	if im.IsVisible() {
		t.Error("expected not visible after Close")
	}
	if v2 := im.View(); v2.Content != "" {
		t.Errorf("expected empty view when closed, got %q", v2.Content)
	}

	// 5. Toggle
	im.Toggle(80, 24)
	if !im.IsVisible() {
		t.Error("expected visible after Toggle from closed")
	}
	im.Toggle(80, 24)
	if im.IsVisible() {
		t.Error("expected not visible after Toggle from open")
	}
}

// TestInfoModalViewportMatchesFrameForVariedBorders sweeps modalFrameStyle
// across several border/padding combinations (standing in for border
// "thickness" — a wider Padding stresses the same GetHorizontalFrameSize /
// GetVerticalFrameSize arithmetic as a thicker border would) and checks
// vpDims() against an independently computed expectation. If vpDims ever
// reverts to hardcoded literals (boxW-4, boxH-6) instead of deriving from the
// live frame style, this fails the moment the frame stops being exactly
// 1-cell-border + Padding(0,1) — which is exactly what changes once a themed
// border option lands. It also renders the full modal and checks no line
// overflows the box, so a viewport sized larger than its frame allows is
// caught too.
func TestInfoModalViewportMatchesFrameForVariedBorders(t *testing.T) {
	original := modalFrameStyle
	t.Cleanup(func() { modalFrameStyle = original })

	cases := []struct {
		name       string
		border     lipgloss.Border
		padV, padH int
	}{
		{"rounded-default", lipgloss.RoundedBorder(), 0, 1},
		{"thick-wide-padding", lipgloss.ThickBorder(), 1, 3},
		{"double-tall-padding", lipgloss.DoubleBorder(), 2, 2},
		{"normal-no-padding", lipgloss.NormalBorder(), 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			padV, padH := tc.padV, tc.padH
			modalFrameStyle = func() lipgloss.Style {
				return lipgloss.NewStyle().Border(tc.border).Padding(padV, padH)
			}

			im := NewInfoModal()
			im.SetAppName("TestApp")
			im.SetVersion("v1.2.3")
			im.Open(100, 40)
			_ = theme.SetCurrentTint("dracula")

			boxW, boxH, _, _ := im.boxDims()
			// Every standard lipgloss border contributes exactly 1 cell per
			// edge; padH/padV apply to both sides, matching Padding(v, h).
			wantVpW := max(boxW-2*padH-2, 10)
			wantVpH := max(boxH-2*padV-2-modalChromeRows, 1)

			gotVpW, gotVpH := im.vpDims()
			if gotVpW != wantVpW || gotVpH != wantVpH {
				t.Fatalf("%s: vpDims() = (%d, %d), want (%d, %d) for boxW=%d boxH=%d padV=%d padH=%d",
					tc.name, gotVpW, gotVpH, wantVpW, wantVpH, boxW, boxH, padV, padH)
			}

			v := im.View()
			for i, line := range strings.Split(v.Content, "\n") {
				if w := lipgloss.Width(line); w > boxW {
					t.Errorf("%s: line %d overflows box width %d by %d: %q", tc.name, i, boxW, w-boxW, line)
				}
			}
		})
	}
}

func TestInfoModal_ResizeAndScrolls(t *testing.T) {
	im := NewInfoModal()
	im.Open(80, 24)

	// Resize while open should rebuild content
	im.Resize(100, 30)
	if im.availableW != 100 || im.availableH != 30 {
		t.Errorf("resize failed to update dims: %d, %d", im.availableW, im.availableH)
	}

	// Scroll methods should not panic
	im.ScrollUp()
	im.ScrollDown()
	im.PageUp()
	im.PageDown()
	im.GotoTop()
	im.GotoBottom()
}

func TestInfoModal_UpdateKeys(t *testing.T) {
	im := NewInfoModal()
	keyMap := keys.DefaultKeyMap()
	im.SetKeys(keyMap)
	im.Open(80, 24)

	// 1. Esc should close and emit CloseInfoModalMsg
	m, cmd := im.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected cmd on Esc")
	}
	msg := cmd()
	if _, ok := msg.(CloseInfoModalMsg); !ok {
		t.Errorf("expected CloseInfoModalMsg, got %T", msg)
	}
	updated, ok := m.(*InfoModal)
	if !ok {
		t.Fatalf("expected *InfoModal, got %T", m)
	}
	if updated.IsVisible() {
		t.Error("expected modal closed after Esc")
	}

	// Reopen
	im.Open(80, 24)

	// 2. Navigation keys should update viewport scroll and not return command
	navKeys := []key.Binding{
		keyMap.Up,
		keyMap.Down,
		keyMap.PageUp,
		keyMap.PageDown,
		keyMap.Top,
		keyMap.Bottom,
	}

	for _, kb := range navKeys {
		if len(kb.Keys()) > 0 {
			kName := kb.Keys()[0]
			_, cmd2 := im.Update(tea.KeyPressMsg{Text: kName})
			if cmd2 != nil {
				t.Errorf("expected nil cmd for navigation key %s", kName)
			}
		}
	}

	// Resize msg in Update
	_, cmdResize := im.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	if cmdResize != nil {
		t.Error("expected nil cmd on WindowSizeMsg")
	}
	if im.availableW != 120 || im.availableH != 50 {
		t.Errorf("expected updated dimensions, got w=%d h=%d", im.availableW, im.availableH)
	}
}
