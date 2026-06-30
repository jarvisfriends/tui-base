package navigation

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestMouseInput(t *testing.T) {
	nav := New()
	// set a width/height so the view is rendered predictably
	nCheck, cCheck := nav.Update(tea.WindowSizeMsg{Width: 20, Height: 10})
	if nCheck != nav || cCheck != nil {
		t.Fatalf("unexpected return from Update: %T %v, %T %v", nCheck, nCheck, cCheck, cCheck)
	}
	v := nav.View()
	lines := strings.Split(v.Content, "\n")
	if len(lines) == 0 {
		t.Fatal("expected non-empty view content")
	}
	fmt.Println("View content lines:")
	for i, line := range lines {
		fmt.Printf("%d: %s\n", i, line)
	}

	for _, page := range nav.Pages {
		for y, line := range lines {
			if !strings.Contains(line, page.Title) {
				continue
			}
			fmt.Printf("Simulating click on page %s at y=%d\n", page.Title, y)
			mm := tea.MouseReleaseMsg{X: 0, Y: y, Button: tea.MouseLeft}
			cmd := v.OnMouse(mm)
			if cmd == nil {
				fmt.Println("OnMouse returned nil cmd")
				break
			}
			msg := cmd()
			sel, ok := msg.(SelectedMsg)
			if ok {
				fmt.Printf("Received SelectedMsg: %+v\n", sel)
			} else {
				fmt.Printf("Received unexpected msg: %T %+v\n", msg, msg)
			}
			fmt.Printf("nav.ActiveIndex=%d\n", nav.ActiveIndex)
			break
		}
	}
}
