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

// PageEnterer is implemented by page models that want a lifecycle hook when
// they become the active page (I-1). The router calls OnEnter after the
// switch — on startup for the initial page, and on every navigation
// afterwards (Tab cycling, sidebar/tab selection, number keys, Ctrl+G,
// status-bar clicks). Returned Cmds run like any other page Cmd, so pages
// can kick off refreshes exactly when they come into view.
//
// The other Q-16 hooks are covered by existing mechanisms rather than new
// interfaces: resize by the router's size forwarding (WindowSizeMsg /
// Component.SetSize) and theme changes by styles.ColorAware plus the shared
// palette pointer.
type PageEnterer interface {
	OnEnter() tea.Cmd
}

// PageLeaver is implemented by page models that want a lifecycle hook when
// they stop being the active page. The router calls OnLeave before OnEnter
// fires on the incoming page; pages can pause tickers or drop caches while
// hidden. Leaving does not fire on shutdown — quit is not a page switch.
type PageLeaver interface {
	OnLeave() tea.Cmd
}
