// Package notifications provides a severity-aware notification manager with
// toast display, history, and optional JSON persistence.
package notifications

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Severity represents the importance level of a notification.
type Severity int

const (
	SeverityInfo    Severity = iota // blue / informational
	SeverityWarning                 // amber / caution
	SeverityError                   // red / critical
)

// String returns a human-readable label.
func (s Severity) String() string {
	switch s {
	case SeverityWarning:
		return "Warning"
	case SeverityError:
		return "Error"
	default:
		return "Info"
	}
}

// ColorForSeverity returns a hex color string for the severity (for use with lipgloss.Color()).
func ColorForSeverity(s Severity) string {
	switch s {
	case SeverityWarning:
		return "#F9C513"
	case SeverityError:
		return "#FF5757"
	default:
		return "#4FC3F7"
	}
}

// Badge returns a 4-character badge string (padded with spaces).
func (s Severity) Badge() string {
	switch s {
	case SeverityWarning:
		return "WARN"
	case SeverityError:
		return "ERR "
	default:
		return "INFO"
	}
}

// DefaultTTL returns a sensible auto-dismiss duration per severity level.
func (s Severity) DefaultTTL() time.Duration {
	switch s {
	case SeverityWarning:
		return 10 * time.Second
	case SeverityError:
		return 15 * time.Second
	default:
		return 5 * time.Second
	}
}

// Notification is one user-facing notification entry.
type Notification struct {
	ID        int64     `json:"id"`
	Content   string    `json:"content"`
	Severity  Severity  `json:"severity"`
	CreatedAt time.Time `json:"created_at"`
	Dismissed bool      `json:"dismissed"`
}

// ─── Messages ──────────────────────────────────────────────────────────────

// AddMsg requests a new notification. TTL=0 means no auto-dismiss.
type AddMsg struct {
	Content  string
	Severity Severity
	TTL      time.Duration
}

// DismissMsg dismisses a specific notification by ID.
type DismissMsg struct{ ID int64 }

// DismissAllMsg dismisses all notifications matching the given severity.
// Leave Severity as nil to dismiss everything.
type DismissAllMsg struct{ Severity *Severity }

// ExpireMsg is delivered by the auto-dismiss Cmd produced when TTL > 0.
// It is exported so the router can route it through notifMgr.Handle().
type ExpireMsg struct{ ID int64 }

// ─── Manager ───────────────────────────────────────────────────────────────

// Manager is a goroutine-safe notification store.
// Pass a single *Manager pointer to all components that need to read or write
// notifications; this follows the same shared-pointer pattern as AppColors.
type Manager struct {
	mu          sync.Mutex
	items       []Notification
	enabled     bool
	nextID      int64
	persistPath string // empty → no file persistence
}

// NewManager creates a Manager with notifications enabled.
func NewManager() *Manager {
	return &Manager{enabled: true}
}

// Enabled reports whether new notifications are accepted.
func (m *Manager) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

// SetEnabled enables or disables the notification system.
// When disabled, Add() is a no-op and the bell icon changes to 🔕.
func (m *Manager) SetEnabled(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = v
}

// SetPersistPath configures the JSON file used for persistence.
// Call before the first Add() or Load() to activate persistence.
func (m *Manager) SetPersistPath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.persistPath = path
}

// Add inserts a notification and returns a Cmd that expires it after ttl.
// If notifications are disabled, both return values are zero/nil.
func (m *Manager) Add(content string, sev Severity, ttl time.Duration) (Notification, tea.Cmd) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.enabled {
		return Notification{}, nil
	}
	m.nextID++
	n := Notification{
		ID:        m.nextID,
		Content:   content,
		Severity:  sev,
		CreatedAt: time.Now(),
	}
	m.items = append(m.items, n)
	m.sortUnsafe()
	m.persistUnsafe()

	if ttl <= 0 {
		return n, nil
	}
	id := n.ID
	return n, func() tea.Msg {
		time.Sleep(ttl)
		return ExpireMsg{ID: id}
	}
}

// Dismiss marks a notification as dismissed by ID.
func (m *Manager) Dismiss(id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.items {
		if m.items[i].ID == id {
			m.items[i].Dismissed = true
			break
		}
	}
	m.sortUnsafe()
	m.persistUnsafe()
}

// DismissAll marks every notification with the given severity as dismissed.
// Pass nil to dismiss everything.
func (m *Manager) DismissAll(sev *Severity) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.items {
		if sev == nil || m.items[i].Severity == *sev {
			m.items[i].Dismissed = true
		}
	}
	m.sortUnsafe()
	m.persistUnsafe()
}

// Active returns undismissed notifications, newest first.
func (m *Manager) Active() []Notification {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Notification
	for _, n := range m.items {
		if !n.Dismissed {
			out = append(out, n)
		}
	}
	return out
}

// All returns all notifications (including dismissed), newest first.
func (m *Manager) All() []Notification {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Notification, len(m.items))
	copy(result, m.items)
	return result
}

// Count returns the number of undismissed notifications.
func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, item := range m.items {
		if !item.Dismissed {
			n++
		}
	}
	return n
}

// Handle processes notification-related tea messages.
// Call this from the router's Update() so the manager stays in sync.
func (m *Manager) Handle(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case AddMsg:
		_, cmd := m.Add(msg.Content, msg.Severity, msg.TTL)
		return cmd
	case DismissMsg:
		m.Dismiss(msg.ID)
	case DismissAllMsg:
		m.DismissAll(msg.Severity)
	case ExpireMsg:
		m.Dismiss(msg.ID)
	}
	return nil
}

// ─── Persistence ───────────────────────────────────────────────────────────

// Save writes the full notification list to a JSON file inside dir.
// The directory is created if it does not exist.
func (m *Manager) Save(dir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return m.writeFileLocked(filepath.Join(dir, "notifications.json"))
}

// Load reads persisted notifications from dir/notifications.json.
// A missing file is silently ignored.
func (m *Manager) Load(dir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	path := filepath.Join(dir, "notifications.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var items []Notification
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	for _, n := range items {
		if n.ID > m.nextID {
			m.nextID = n.ID
		}
	}
	m.items = items
	m.persistPath = path
	return nil
}

// ─── internal helpers ──────────────────────────────────────────────────────

// sortUnsafe sorts items: undismissed first, then newest first. Caller holds mu.
func (m *Manager) sortUnsafe() {
	sort.SliceStable(m.items, func(i, j int) bool {
		if m.items[i].Dismissed != m.items[j].Dismissed {
			return !m.items[i].Dismissed // undismissed first
		}
		return m.items[i].CreatedAt.After(m.items[j].CreatedAt)
	})
}

// persistUnsafe saves to persistPath if configured. Caller holds mu.
func (m *Manager) persistUnsafe() {
	if m.persistPath == "" {
		return
	}
	_ = m.writeFileLocked(m.persistPath)
}

// writeFileLocked writes items as JSON to path. Caller holds mu.
func (m *Manager) writeFileLocked(path string) error {
	data, err := json.Marshal(m.items)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
