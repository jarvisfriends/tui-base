package navigation

import (
	"fmt"
	"testing"

	"github.com/jarvisfriends/snap/rendercheck"
)

func TestTabsBounds(t *testing.T) {
	tabs := NewTabs()

	// Create "more tabs than can fit on the screen"
	pages := make([]Page, 0, 20)
	for i := range 20 {
		pages = append(pages, Page{ID: fmt.Sprintf("tab%d", i), Title: fmt.Sprintf("Tab %d", i)})
	}
	tabs.Pages = pages

	// terminal size is 78x61
	width := 78
	height := 61

	// Test with first tab selected
	tabs.ActiveIndex = 0
	rendercheck.AssertBounds(t, tabs, width, height)

	// Test with last tab selected
	tabs.ActiveIndex = len(pages) - 1
	rendercheck.AssertBounds(t, tabs, width, height)

	// Test a few middle tabs
	for i := 1; i < len(pages)-1; i++ {
		tabs.ActiveIndex = i
		rendercheck.AssertBounds(t, tabs, width, height)
	}
}
