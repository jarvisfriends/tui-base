// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package settings

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jarvisfriends/snap/datepicker"
)

// TestModelOverlayForwardsMouseToHostedPicker reproduces the live bug where
// clicks on a hosted date/time picker did nothing: the settings OnMouse
// swallowed inside clicks, and the Update path forwarded them with
// page-relative coordinates that missed every hit zone. Mouse must reach the
// hosted model exactly once, translated into its content space.
func TestModelOverlayForwardsMouseToHostedPicker(t *testing.T) {
	m := NewWithOptions(Options{})
	m.SetSize(100, 40)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	dp := datepicker.New(time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC))
	_ = m.modelOverlay.Open(dp, 100, 40)
	// Composite computes the overlay bounds; the picker's View records its
	// own hit zones.
	_ = m.modelOverlay.Composite(stubBase(100, 40), stubBorder())
	_ = dp.View()

	// Page-relative coordinates of day 15's cell = overlay origin + content
	// inset + the picker's own recorded cell position.
	bounds := m.modelOverlay.Bounds()
	cellX, cellY := datepickerCellPos(t, dp, 15)
	pageX := bounds.X + 3 + cellX // 3 = border+padding inset (overlay pkg)
	pageY := bounds.Y + 2 + cellY

	onMouse := m.View().OnMouse
	if onMouse == nil {
		t.Fatal("settings view must set OnMouse")
	}
	_ = onMouse(tea.MouseClickMsg{X: pageX, Y: pageY, Button: tea.MouseLeft})
	if dp.Time.Day() != 15 {
		t.Fatalf("click through the overlay moved highlight to day %d; want 15", dp.Time.Day())
	}

	// The Update path must NOT double-deliver mouse while the overlay is open
	// (a raw page-relative click could hit the wrong cell).
	before := dp.Time
	_, _ = m.Update(tea.MouseClickMsg{X: 2, Y: 2, Button: tea.MouseLeft})
	if !dp.Time.Equal(before) {
		t.Fatalf("Update path leaked a raw mouse click into the hosted picker (%v -> %v)",
			before, dp.Time)
	}

	// Wheel forwards too (pages weeks in the datepicker).
	_ = onMouse(tea.MouseWheelMsg{X: pageX, Y: pageY, Button: tea.MouseWheelDown})
	if dp.Time.Day() != 22 {
		t.Fatalf("wheel through the overlay moved to day %d; want 22", dp.Time.Day())
	}
}

// datepickerCellPos digs the recorded content-relative position of a day cell
// out of the picker (same math as its own click handler, driven via public
// behavior: probe clicks until the day matches).
func datepickerCellPos(t *testing.T, dp *datepicker.DatePickerModel, day int) (x, y int) {
	t.Helper()
	// Probe the rendered grid: try every cell position via a scratch picker
	// so we don't disturb dp's state.
	probe := datepicker.New(dp.Time)
	view := probe.View()
	_ = view
	for py := range 30 {
		for px := range 60 {
			scratch := datepicker.New(dp.Time)
			// View records the hit zones and carries OnMouse — the only
			// door mouse goes through now.
			_ = scratch.View().OnMouse(tea.MouseClickMsg{X: px, Y: py, Button: tea.MouseLeft})
			if scratch.Time.Day() == day && scratch.Focused == datepicker.FocusCalendar {
				return px, py
			}
		}
	}
	t.Fatalf("no clickable cell found for day %d", day)
	return 0, 0
}

func stubBorder() lipgloss.Style { return lipgloss.NewStyle() }

func stubBase(w, h int) string {
	line := strings.Repeat(" ", w)
	rows := make([]string, h)
	for i := range rows {
		rows[i] = line
	}
	return strings.Join(rows, "\n")
}
