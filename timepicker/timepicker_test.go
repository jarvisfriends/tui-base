package timepicker

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// send dispatches a message through Update and returns the updated model.
func send(m *TimePickerModel, msg tea.Msg) *TimePickerModel {
	next, _ := m.Update(msg)
	r, ok := next.(*TimePickerModel)
	if !ok {
		panic(fmt.Sprintf("Update returned %T, want *TimePickerModel", next))
	}
	return r
}

func TestNew(t *testing.T) {
	t.Parallel()

	d := 2*time.Hour + 30*time.Minute + 15*time.Second
	m := New(d)

	if m.Duration != d {
		t.Errorf("Duration = %v; want %v", m.Duration, d)
	}
	if m.Focused != FieldHours {
		t.Errorf("Focused = %v; want FieldHours", m.Focused)
	}
	if m.Done {
		t.Error("Done should be false on construction")
	}
	if m.Aborted {
		t.Error("Aborted should be false on construction")
	}
}

func TestInitReturnsNil(t *testing.T) {
	t.Parallel()
	m := New(0)
	if m.Init() != nil {
		t.Error("Init() should return nil")
	}
}

func TestDefaultKeyMapBindings(t *testing.T) {
	t.Parallel()
	km := DefaultKeyMap()
	if len(km.Up.Keys()) == 0 {
		t.Error("Up binding has no keys")
	}
	if len(km.Submit.Keys()) == 0 {
		t.Error("Submit binding has no keys")
	}
	if len(km.Quit.Keys()) == 0 {
		t.Error("Quit binding has no keys")
	}
}

// ── Navigation (Left/Right) ─────────────────────────────────────────────────

func TestRightKeyAdvancesField(t *testing.T) {
	t.Parallel()
	m := New(0)
	m = send(m, tea.KeyPressMsg{Code: tea.KeyRight})
	if m.Focused != FieldMinutes {
		t.Errorf("after Right: Focused = %v; want FieldMinutes", m.Focused)
	}
	m = send(m, tea.KeyPressMsg{Code: tea.KeyRight})
	if m.Focused != FieldSeconds {
		t.Errorf("after 2xRight: Focused = %v; want FieldSeconds", m.Focused)
	}
	// Wrap from seconds back to hours.
	m = send(m, tea.KeyPressMsg{Code: tea.KeyRight})
	if m.Focused != FieldHours {
		t.Errorf("after wrap-right: Focused = %v; want FieldHours", m.Focused)
	}
}

func TestLeftKeyReversesField(t *testing.T) {
	t.Parallel()
	m := New(0)
	// Wrap left from hours to seconds.
	m = send(m, tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.Focused != FieldSeconds {
		t.Errorf("after wrap-left: Focused = %v; want FieldSeconds", m.Focused)
	}
	m = send(m, tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.Focused != FieldMinutes {
		t.Errorf("after 2xLeft: Focused = %v; want FieldMinutes", m.Focused)
	}
}

func TestTabKeyAdvancesField(t *testing.T) {
	t.Parallel()
	m := New(0)
	m = send(m, tea.KeyPressMsg{Text: "tab"})
	if m.Focused != FieldMinutes {
		t.Errorf("after tab: Focused = %v; want FieldMinutes", m.Focused)
	}
}

func TestShiftTabKeyReversesField(t *testing.T) {
	t.Parallel()
	m := New(0)
	m = send(m, tea.KeyPressMsg{Text: "shift+tab"})
	if m.Focused != FieldSeconds {
		t.Errorf("after shift+tab: Focused = %v; want FieldSeconds", m.Focused)
	}
}

// ── Submit / Abort ───────────────────────────────────────────────────────────

func TestEnterKeySetsDone(t *testing.T) {
	t.Parallel()
	m := New(0)
	m = send(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.Done {
		t.Error("Enter key should set Done = true")
	}
	if m.Aborted {
		t.Error("Enter key should not set Aborted")
	}
}

func TestQuitKeysSetAborted(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"ctrl+c", "esc", "q"} {
		m := New(0)
		m = send(m, tea.KeyPressMsg{Text: k})
		if !m.Aborted {
			t.Errorf("key %q should set Aborted = true", k)
		}
		if m.Done {
			t.Errorf("key %q should not set Done", k)
		}
	}
}

// ── Increment / Decrement ────────────────────────────────────────────────────

func TestUpKeyIncrementsHours(t *testing.T) {
	t.Parallel()
	m := New(0)
	m = send(m, tea.KeyPressMsg{Code: tea.KeyUp})
	if m.Duration != time.Hour {
		t.Errorf("Up on hours: Duration = %v; want 1h", m.Duration)
	}
}

func TestDownKeyHoursFloorAtZero(t *testing.T) {
	t.Parallel()
	m := New(0)
	m = send(m, tea.KeyPressMsg{Code: tea.KeyDown})
	if m.Duration != 0 {
		t.Errorf("Down on hours at 0: Duration = %v; want 0", m.Duration)
	}
}

func TestUpKeyIncrementsMinutes(t *testing.T) {
	t.Parallel()
	m := New(0)
	m.Focused = FieldMinutes
	m = send(m, tea.KeyPressMsg{Code: tea.KeyUp})
	if m.Duration != time.Minute {
		t.Errorf("Up on minutes: Duration = %v; want 1m", m.Duration)
	}
}

func TestMinutesWrapForward(t *testing.T) {
	t.Parallel()
	m := New(59 * time.Minute)
	m.Focused = FieldMinutes
	m = send(m, tea.KeyPressMsg{Code: tea.KeyUp})
	// 59m + 1 → 0m and carry 1h
	want := time.Hour
	if m.Duration != want {
		t.Errorf("minutes wrap forward: Duration = %v; want %v", m.Duration, want)
	}
}

func TestMinutesWrapBackward(t *testing.T) {
	t.Parallel()
	m := New(time.Hour) // 1h 00m 00s
	m.Focused = FieldMinutes
	m = send(m, tea.KeyPressMsg{Code: tea.KeyDown})
	// 0m - 1 → 59m and borrow 1h → 0h 59m
	want := 59 * time.Minute
	if m.Duration != want {
		t.Errorf("minutes wrap backward: Duration = %v; want %v", m.Duration, want)
	}
}

func TestMinutesDownAtZeroNoHoursBorrow(t *testing.T) {
	t.Parallel()
	// 0h 0m: decrementing minutes should not borrow from hours (hours already 0)
	m := New(0)
	m.Focused = FieldMinutes
	m = send(m, tea.KeyPressMsg{Code: tea.KeyDown})
	// 0m - 1, but hours==0 so hours stays 0; only minutes wrap
	wantMinutes := int64(59)
	gotMinutes := int64(m.Duration.Minutes()) % 60
	if gotMinutes != wantMinutes {
		t.Errorf("minutes wrap at 0 hours: minutes = %d; want %d", gotMinutes, wantMinutes)
	}
}

func TestSecondsWrapForward(t *testing.T) {
	t.Parallel()
	m := New(59 * time.Second)
	m.Focused = FieldSeconds
	m = send(m, tea.KeyPressMsg{Code: tea.KeyUp})
	// 59s + 1 → 0s carry 1m
	want := time.Minute
	if m.Duration != want {
		t.Errorf("seconds wrap forward: Duration = %v; want %v", m.Duration, want)
	}
}

func TestSecondsWrapForwardWithMinuteCarry(t *testing.T) {
	t.Parallel()
	m := New(59*time.Minute + 59*time.Second)
	m.Focused = FieldSeconds
	m = send(m, tea.KeyPressMsg{Code: tea.KeyUp})
	// 59s + 1 → 0s, 59m + 1 → 0m, carry 1h
	want := time.Hour
	if m.Duration != want {
		t.Errorf("seconds→minutes→hours carry: Duration = %v; want %v", m.Duration, want)
	}
}

func TestSecondsWrapBackward(t *testing.T) {
	t.Parallel()
	m := New(time.Minute) // 0h 1m 0s
	m.Focused = FieldSeconds
	m = send(m, tea.KeyPressMsg{Code: tea.KeyDown})
	// 0s - 1 → 59s, borrow 1m
	want := 59 * time.Second
	if m.Duration != want {
		t.Errorf("seconds wrap backward: Duration = %v; want %v", m.Duration, want)
	}
}

func TestSecondsWrapBackwardBorrowFromHours(t *testing.T) {
	t.Parallel()
	m := New(time.Hour) // 1h 0m 0s
	m.Focused = FieldSeconds
	m = send(m, tea.KeyPressMsg{Code: tea.KeyDown})
	// 0s - 1 → 59s, borrow 1m; 0m → 59m, borrow 1h
	want := 59*time.Minute + 59*time.Second
	if m.Duration != want {
		t.Errorf("seconds borrow from hours: Duration = %v; want %v", m.Duration, want)
	}
}

func TestSecondsDownAtZeroNoBorrow(t *testing.T) {
	t.Parallel()
	// 0h 0m: decrementing seconds should wrap minutes to 59 (can't borrow hours)
	m := New(0)
	m.Focused = FieldSeconds
	m = send(m, tea.KeyPressMsg{Code: tea.KeyDown})
	// 0s - 1, 0m (no borrow possible from hours=0), minutes stay 0, seconds wrap to 59
	wantSeconds := int64(59)
	gotSeconds := int64(m.Duration.Seconds()) % 60
	if gotSeconds != wantSeconds {
		t.Errorf("seconds wrap at 0 minutes/hours: seconds = %d; want %d", gotSeconds, wantSeconds)
	}
}

// ── Mouse wheel ──────────────────────────────────────────────────────────────

func TestMouseWheelUpIncrements(t *testing.T) {
	t.Parallel()
	m := New(0)
	m = send(m, tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelUp}))
	if m.Duration != time.Hour {
		t.Errorf("mouse wheel up: Duration = %v; want 1h", m.Duration)
	}
}

func TestMouseWheelDownDecrements(t *testing.T) {
	t.Parallel()
	m := New(time.Hour)
	m = send(m, tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
	if m.Duration != 0 {
		t.Errorf("mouse wheel down from 1h: Duration = %v; want 0", m.Duration)
	}
}

// ── View ────────────────────────────────────────────────────────────────────

func TestViewContainsTimeComponents(t *testing.T) {
	t.Parallel()
	m := New(2*time.Hour + 30*time.Minute + 5*time.Second)
	v := m.View()
	content := v.Content
	if !strings.Contains(content, "02h") {
		t.Errorf("View missing hours: %q", content)
	}
	if !strings.Contains(content, "30m") {
		t.Errorf("View missing minutes: %q", content)
	}
	if !strings.Contains(content, "05s") {
		t.Errorf("View missing seconds: %q", content)
	}
}

func TestViewZeroDuration(t *testing.T) {
	t.Parallel()
	m := New(0)
	v := m.View()
	if v.Content == "" {
		t.Error("View() should return non-empty content for zero duration")
	}
	if !strings.Contains(v.Content, "00h") {
		t.Errorf("View missing zero hours: %q", v.Content)
	}
}
