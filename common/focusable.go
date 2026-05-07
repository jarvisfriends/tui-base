package common

import tea "charm.land/bubbletea/v2"

type Focusable interface {
	Focus() tea.Cmd
	Blur()
	Update(tea.Msg) (tea.Model, tea.Cmd)
	Init() tea.Cmd
	View() tea.View
}

var (
	_ tea.Model = (Focusable)(nil)
	_ tea.Model = (*KnownFocusable)(nil)
)

type KnownFocusable struct {
	focusedIndex int
	focusables   []Focusable
}

// Init implements [tea.Model].
func (k *KnownFocusable) Init() tea.Cmd {
	k.focusedIndex = 0
	if len(k.focusables) > 0 {
		return k.focusables[0].Focus()
	}
	return nil
}

// View implements [tea.Model].
func (k *KnownFocusable) View() tea.View {
	if len(k.focusables) == 0 {
		return tea.NewView("")
	}
	return k.focusables[k.focusedIndex].View()

}

func (k *KnownFocusable) Next() tea.Cmd {
	if len(k.focusables) == 0 {
		return nil
	}
	k.focusables[k.focusedIndex].Blur()
	k.focusedIndex = (k.focusedIndex + 1) % len(k.focusables)
	return k.focusables[k.focusedIndex].Focus()
}

func (k *KnownFocusable) Prev() tea.Cmd {
	if len(k.focusables) == 0 {
		return nil
	}
	k.focusables[k.focusedIndex].Blur()
	k.focusedIndex = (k.focusedIndex - 1 + len(k.focusables)) % len(k.focusables)
	return k.focusables[k.focusedIndex].Focus()
}

func (k *KnownFocusable) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(k.focusables) == 0 {
		return k, nil
	}
	updated, cmd := k.focusables[k.focusedIndex].Update(msg)
	k.focusables[k.focusedIndex] = updated.(Focusable)
	return k, cmd
}

func (k *KnownFocusable) Add(focusable Focusable) {
	k.focusables = append(k.focusables, focusable)
}

func (k *KnownFocusable) Focus() tea.Cmd {
	if len(k.focusables) == 0 {
		return nil
	}
	return k.focusables[k.focusedIndex].Focus()
}

func (k *KnownFocusable) Blur() {
	if len(k.focusables) == 0 {
		return
	}
	k.focusables[k.focusedIndex].Blur()
}
