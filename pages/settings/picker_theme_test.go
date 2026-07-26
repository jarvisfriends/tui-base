// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package settings

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/snap/timepicker"
	"github.com/jarvisfriends/tui-base/theme"
)

// TestThemedDatePickerStylesUseSelectionColors verifies the datepicker's
// focused cell derives from the theme's semantic selection colors
// instead of the component's hardcoded ANSI defaults.
func TestThemedDatePickerStylesUseSelectionColors(t *testing.T) {
	t.Parallel()

	c := theme.Active()
	s := themedDatePickerStyles(c)
	if got := s.FocusedText.GetBackground(); got != c.SelectionBg {
		t.Errorf("FocusedText background = %v; want theme SelectionBg %v", got, c.SelectionBg)
	}
	if got := s.FocusedText.GetForeground(); got != c.SelectionFg {
		t.Errorf("FocusedText foreground = %v; want theme SelectionFg %v", got, c.SelectionFg)
	}
}

// TestTimePickerThemeUsesAccent verifies the timepicker's focused segment
// derives from the theme accent.
func TestTimePickerThemeUsesAccent(t *testing.T) {
	t.Parallel()

	c := theme.Active()
	tp := timepicker.New(time.Minute)
	applyTimePickerTheme(tp, c)
	if got := tp.ActiveStyle.GetForeground(); got != c.Accent {
		t.Errorf("ActiveStyle foreground = %v; want theme Accent %v", got, c.Accent)
	}
}

// hardcodedPink is the SGR parameter of the old hardcoded ANSI-256 color 212
// that used to mark selections in the small editor overlays.
const hardcodedPink = "38;5;212"

// TestEditorOverlaysUseThemeNotHardcodedColors renders the small editor
// overlays and asserts the old hardcoded selection color is gone.
func TestEditorOverlaysUseThemeNotHardcodedColors(t *testing.T) {
	t.Parallel()

	dp := newThemedDirPicker("")
	dp.Width, dp.Height = 80, 24
	if cmd := dp.Init(); cmd != nil {
		_, _ = dp.Update(cmd())
	}
	if strings.Contains(dp.View().Content, hardcodedPink) {
		t.Error("DirPicker still renders the hardcoded 212 selection color")
	}

	mf := newThemedMultiFileEditor("a; b")
	_, _ = mf.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if strings.Contains(mf.View().Content, hardcodedPink) {
		t.Error("MultiFileEditor still renders the hardcoded 212 selection color")
	}

	kr := NewKeyRecorder("x")
	if strings.Contains(kr.View().Content, hardcodedPink) {
		t.Error("KeyRecorder still renders the hardcoded 212 selection color")
	}
}
