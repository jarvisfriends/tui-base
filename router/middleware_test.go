package router

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

const (
	RecorderPageTitle = "Recorder"
)

type gatedMsg struct{}

type translatedFromMsg struct{}

type translatedToMsg struct{}

// TestMessageMiddleware verifies E-7: middleware observes every message in
// registration order, can swallow a message by returning nil, and can replace
// one message with another before routing.
func TestMessageMiddleware(t *testing.T) {
	t.Parallel()

	rec := &mouseRecorderPage{}
	m := NewWithRegisteredPages([]RegisteredPage{{Title: RecorderPageTitle, Model: rec}})
	_, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 30})

	var seen []tea.Msg
	m.Use(func(msg tea.Msg) tea.Msg {
		seen = append(seen, msg)
		return msg
	})
	m.Use(func(msg tea.Msg) tea.Msg {
		switch msg.(type) {
		case gatedMsg:
			return nil // swallowed: nothing downstream sees it
		case translatedFromMsg:
			return translatedToMsg{}
		}
		return msg
	})

	translatedSeen := false
	m.Use(func(msg tea.Msg) tea.Msg {
		if _, ok := msg.(translatedToMsg); ok {
			translatedSeen = true
		}
		return msg
	})

	_, _ = m.Update(gatedMsg{})
	if len(seen) != 1 {
		t.Fatalf("observer middleware saw %d messages; want 1", len(seen))
	}

	_, _ = m.Update(translatedFromMsg{})
	if !translatedSeen {
		t.Fatal("translation was not visible to later middleware")
	}
	if len(seen) != 2 {
		t.Fatalf("observer middleware saw %d messages; want 2", len(seen))
	}
}
