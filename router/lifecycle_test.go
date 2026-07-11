package router

import (
	"testing"

	"github.com/jarvisfriends/snap/navigation"
	"github.com/jarvisfriends/snap/page"

	tea "charm.land/bubbletea/v2"
)

// lifecyclePage records the I-1 OnEnter/OnLeave hook calls the router makes.
type lifecyclePage struct {
	page.Base
	enters, leaves int
}

func (p *lifecyclePage) Init() tea.Cmd                       { return nil }
func (p *lifecyclePage) Update(tea.Msg) (tea.Model, tea.Cmd) { return p, nil }
func (p *lifecyclePage) View() tea.View                      { return tea.NewView("lifecycle") }
func (p *lifecyclePage) OnEnter() tea.Cmd                    { p.enters++; return nil }
func (p *lifecyclePage) OnLeave() tea.Cmd                    { p.leaves++; return nil }

// drainCmd executes a Cmd tree so hook Cmds batched with resize Cmds run.
func drainCmd(m *RouterModel, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	switch batch := msg.(type) {
	case tea.BatchMsg:
		for _, c := range batch {
			drainCmd(m, c)
		}
	default:
		if msg != nil {
			_, next := m.Update(msg)
			_ = next
		}
	}
}

// newLifecycleRouter returns a router whose first page records lifecycle
// calls, plus that page.
func newLifecycleRouter(t *testing.T) (*RouterModel, *lifecyclePage) {
	t.Helper()
	pg := &lifecyclePage{}
	m := NewWithOptions(Options{
		ConfigDir:  t.TempDir(),
		ExtraPages: []RegisteredPage{{Title: "Lifecycle", Model: pg}},
	})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m, pg
}

// TestLifecycleInitialPageEnters: the startup page gets OnEnter once, from
// Init, after page Inits.
func TestLifecycleInitialPageEnters(t *testing.T) {
	m, pg := newLifecycleRouter(t)
	drainCmd(m, m.Init())
	if pg.enters != 1 {
		t.Fatalf("initial OnEnter: got %d want 1", pg.enters)
	}
	if pg.leaves != 0 {
		t.Fatalf("initial OnLeave: got %d want 0", pg.leaves)
	}
}

// TestLifecycleCycleFiresLeaveThenEnter: cycling away from and back to the
// page fires OnLeave and OnEnter around each switch.
func TestLifecycleCycleFiresLeaveThenEnter(t *testing.T) {
	m, pg := newLifecycleRouter(t)

	drainCmd(m, m.cyclePage(1)) // Lifecycle -> Settings
	if pg.leaves != 1 {
		t.Fatalf("OnLeave after cycling away: got %d want 1", pg.leaves)
	}
	if pg.enters != 0 {
		t.Fatalf("OnEnter should not fire when leaving: got %d", pg.enters)
	}

	drainCmd(m, m.cyclePage(1)) // Settings -> Lifecycle (2 pages, wraps)
	if pg.enters != 1 {
		t.Fatalf("OnEnter after cycling back: got %d want 1", pg.enters)
	}
}

// TestLifecycleActivateByIDAndSelectedMsg: Ctrl+G-style activation and a nav
// SelectedMsg both route through the hooks; re-selecting the active page
// fires nothing.
func TestLifecycleActivateByIDAndSelectedMsg(t *testing.T) {
	m, pg := newLifecycleRouter(t)

	if cmd, ok := m.activatePageByID(navigation.PageIDSettings); !ok {
		t.Fatal("settings page not found")
	} else {
		drainCmd(m, cmd)
	}
	if pg.leaves != 1 {
		t.Fatalf("OnLeave via activatePageByID: got %d want 1", pg.leaves)
	}

	_, cmd := m.Update(navigation.SelectedMsg{PageIndex: 0})
	drainCmd(m, cmd)
	if pg.enters != 1 {
		t.Fatalf("OnEnter via SelectedMsg: got %d want 1", pg.enters)
	}

	// Re-selecting the already-active page is not a page change.
	_, cmd = m.Update(navigation.SelectedMsg{PageIndex: 0})
	drainCmd(m, cmd)
	if pg.enters != 1 || pg.leaves != 1 {
		t.Fatalf("re-select fired hooks: enters=%d leaves=%d", pg.enters, pg.leaves)
	}
}
