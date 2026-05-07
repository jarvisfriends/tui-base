package common

import (
	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
)

// Component represents a Bubble Tea model that implements a SetSize function.
type Component interface {
	tea.Model
	help.KeyMap
	SetSize(width, height int)
}
