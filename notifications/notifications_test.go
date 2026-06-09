package notifications_test

import (
	"path/filepath"
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

func TestManager_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	m := notifications.NewManager()

	// 1. Initial Load from empty dir should do nothing
	if err := m.Load(tmpDir); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if m.Count() != 0 {
		t.Errorf("expected 0 loaded notifications, got %d", m.Count())
	}

	// 2. Add notification and Save
	m.Add("notif1", notifications.SeverityInfo, 0)
	time.Sleep(10 * time.Millisecond) // ensure different timestamps
	m.Add("notif2", notifications.SeverityWarning, 0)
	if err := m.Save(tmpDir); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// 3. Load in a new manager
	m2 := notifications.NewManager()
	if err := m2.Load(tmpDir); err != nil {
		t.Fatalf("load m2 failed: %v", err)
	}
	if m2.Count() != 2 {
		t.Errorf("expected 2 loaded notifications in m2, got %d", m2.Count())
	}
	active := m2.Active()
	if active[0].Content != "notif2" || active[1].Content != "notif1" {
		t.Errorf("unexpected loaded order or contents: %+v", active)
	}

	// 4. Test SetPersistPath auto-saves on Add/Dismiss
	// Using filepath.Join with "notifications.json" since Load(dir) expects that exact filename.
	autoFile := filepath.Join(tmpDir, "notifications.json")

	m3 := notifications.NewManager()
	m3.SetPersistPath(autoFile)
	n, _ := m3.Add("auto-save", notifications.SeverityError, 0)

	// Load back in m4 from path
	m4 := notifications.NewManager()
	m4.SetPersistPath(autoFile)
	if err := m4.Load(tmpDir); err != nil {
		t.Fatalf("load autoFile failed: %v", err)
	}
	if m4.Count() != 1 || m4.Active()[0].Content != "auto-save" {
		t.Errorf("auto-save failed to persist to autoFile: %+v", m4.Active())
	}

	// Dismiss and verify auto-save
	m3.Dismiss(n.ID)
	m5 := notifications.NewManager()
	m5.SetPersistPath(autoFile)
	if err := m5.Load(tmpDir); err != nil {
		t.Fatalf("load autoFile m5 failed: %v", err)
	}
	if m5.Count() != 0 {
		t.Errorf("expected 0 active after auto-save dismiss, got %d", m5.Count())
	}
}

func TestSeverity_StringAndBadge(t *testing.T) {
	tests := []struct {
		s   notifications.Severity
		str string
		bdg string
	}{
		{notifications.SeverityInfo, "Info", "INFO"},
		{notifications.SeverityWarning, "Warning", "WARN"},
		{notifications.SeverityError, "Error", "ERR "},
	}

	for _, tc := range tests {
		if got := tc.s.String(); got != tc.str {
			t.Errorf("%v String() = %q; want %q", tc.s, got, tc.str)
		}
		if got := tc.s.Badge(); got != tc.bdg {
			t.Errorf("%v Badge() = %q; want %q", tc.s, got, tc.bdg)
		}
	}
}

func TestManager_Handle(t *testing.T) {
	m := notifications.NewManager()

	// 1. Handle AddMsg
	cmd := m.Handle(notifications.AddMsg{
		Content:  "handle-add",
		Severity: notifications.SeverityInfo,
		TTL:      0,
	})
	if cmd != nil {
		t.Error("expected nil cmd for TTL=0")
	}
	if m.Count() != 1 || m.Active()[0].Content != "handle-add" {
		t.Errorf("failed to handle AddMsg: %+v", m.Active())
	}

	id := m.Active()[0].ID

	// 2. Handle ExpireMsg (should dismiss)
	m.Handle(notifications.ExpireMsg{ID: id})
	if m.Count() != 0 {
		t.Errorf("expected 0 active after ExpireMsg, got %d", m.Count())
	}

	// 3. Handle AddMsg and DismissMsg
	m.Handle(notifications.AddMsg{
		Content:  "handle-add-2",
		Severity: notifications.SeverityWarning,
	})
	id2 := m.Active()[0].ID
	m.Handle(notifications.DismissMsg{ID: id2})
	if m.Count() != 0 {
		t.Errorf("expected 0 active after DismissMsg, got %d", m.Count())
	}

	// 4. Handle AddMsg and DismissAllMsg
	m.Handle(notifications.AddMsg{
		Content:  "handle-add-3",
		Severity: notifications.SeverityError,
	})
	sev := notifications.SeverityError
	m.Handle(notifications.DismissAllMsg{Severity: &sev})
	if m.Count() != 0 {
		t.Errorf("expected 0 active after DismissAllMsg, got %d", m.Count())
	}

	// 5. Enabled check
	if !m.Enabled() {
		t.Error("expected manager enabled by default")
	}
}
