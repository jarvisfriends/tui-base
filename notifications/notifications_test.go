package notifications_test

import (
	"testing"
	"time"

	"github.com/jarvisfriends/tui-base/notifications"
)

func TestManager_AddAndActive(t *testing.T) {
	m := notifications.NewManager()
	if m.Count() != 0 {
		t.Fatalf("expected 0 notifications, got %d", m.Count())
	}

	n, _ := m.Add("hello", notifications.SeverityInfo, 0)
	if n.ID == 0 {
		t.Fatal("expected non-zero notification ID")
	}
	if m.Count() != 1 {
		t.Fatalf("expected 1 notification, got %d", m.Count())
	}
	active := m.Active()
	if len(active) != 1 || active[0].Content != "hello" {
		t.Fatalf("unexpected active list: %+v", active)
	}
}

func TestManager_Dismiss(t *testing.T) {
	m := notifications.NewManager()
	n, _ := m.Add("test", notifications.SeverityWarning, 0)
	m.Dismiss(n.ID)
	if m.Count() != 0 {
		t.Fatalf("expected 0 active after dismiss, got %d", m.Count())
	}
	if len(m.All()) != 1 {
		t.Fatal("expected 1 in All() after dismiss")
	}
}

func TestManager_DismissAll(t *testing.T) {
	m := notifications.NewManager()
	m.Add("a", notifications.SeverityInfo, 0)    //nolint:errcheck
	m.Add("b", notifications.SeverityWarning, 0) //nolint:errcheck
	m.Add("c", notifications.SeverityInfo, 0)    //nolint:errcheck

	sev := notifications.SeverityInfo
	m.DismissAll(&sev)
	if m.Count() != 1 {
		t.Fatalf("expected 1 active after dismiss-all info, got %d", m.Count())
	}
}

func TestManager_Disabled(t *testing.T) {
	m := notifications.NewManager()
	m.SetEnabled(false)
	n, cmd := m.Add("ignored", notifications.SeverityError, 0)
	if n.ID != 0 || cmd != nil {
		t.Fatal("expected zero notification when disabled")
	}
	if m.Count() != 0 {
		t.Fatalf("expected 0 notifications when disabled, got %d", m.Count())
	}
}

func TestSeverity_DefaultTTL(t *testing.T) {
	if notifications.SeverityInfo.DefaultTTL() != 5*time.Second {
		t.Fatal("unexpected Info TTL")
	}
	if notifications.SeverityWarning.DefaultTTL() != 10*time.Second {
		t.Fatal("unexpected Warning TTL")
	}
	if notifications.SeverityError.DefaultTTL() != 15*time.Second {
		t.Fatal("unexpected Error TTL")
	}
}

func TestColorForSeverity(t *testing.T) {
	if notifications.ColorForSeverity(notifications.SeverityInfo) == "" {
		t.Fatal("expected non-empty color for Info")
	}
	if notifications.ColorForSeverity(notifications.SeverityWarning) == "" {
		t.Fatal("expected non-empty color for Warning")
	}
	if notifications.ColorForSeverity(notifications.SeverityError) == "" {
		t.Fatal("expected non-empty color for Error")
	}
}
