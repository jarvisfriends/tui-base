// Package datepicker provides a bubble tea component for viewing and selecting
// a date from a monthly view.
package datepicker

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Focus is a value passed to `model.SetFocus` to indicate what component
// controls should be available.
type Focus int

const (
	// FocusNone is a value passed to `model.SetFocus` to ignore all date altering key msgs
	FocusNone Focus = iota
	// FocusHeaderMonth is a value passed to `model.SetFocus` to accept key msgs that change the month
	FocusHeaderMonth
	// FocusHeaderYear is a value passed to `model.SetFocus` to accept key msgs that change the year
	FocusHeaderYear
	// FocusCalendar is a value passed to `model.SetFocus` to accept key msgs that change the week or date
	FocusCalendar
)

//go:generate stringer -type=Focus

// KeyMap is the key bindings for different actions within the datepicker.
type KeyMap struct {
	Up        key.Binding
	Right     key.Binding
	Down      key.Binding
	Left      key.Binding
	FocusPrev key.Binding
	FocusNext key.Binding
	Quit      key.Binding
}

// DefaultKeyMap returns a KeyMap struct with default values
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:        key.NewBinding(key.WithKeys("up")),
		Right:     key.NewBinding(key.WithKeys("right")),
		Down:      key.NewBinding(key.WithKeys("down")),
		Left:      key.NewBinding(key.WithKeys("left")),
		FocusPrev: key.NewBinding(key.WithKeys("shift+tab")),
		FocusNext: key.NewBinding(key.WithKeys("tab")),
		Quit:      key.NewBinding(key.WithKeys("ctrl+c", "q")),
	}
}

// Styles is a struct of lipgloss styles to apply to various elements of the datepicker
type Styles struct {
	Header lipgloss.Style
	Date   lipgloss.Style

	HeaderText   lipgloss.Style
	Text         lipgloss.Style
	SelectedText lipgloss.Style
	FocusedText  lipgloss.Style
}

// DefaultStyles returns a default `Styles` struct
func DefaultStyles() Styles {
	// TODO: refactor for adaptive colors
	r := lipgloss.NewStyle()
	return Styles{
		Header:       r.Padding(1, 0, 0),
		Date:         r.Padding(0, 1, 1),
		HeaderText:   r.Bold(true),
		Text:         r.Foreground(lipgloss.Color("247")),
		SelectedText: r.Bold(true),
		FocusedText:  r.Foreground(lipgloss.Color("212")).Bold(true),
	}
}

// Model is a struct that contains the state of the datepicker component and satisfies
// the `tea.Model` interface
type DatePickerModel struct {
	// Time is the `time.Time` struct that represents the selected date month and year
	Time time.Time

	// KeyMap encodes the keybindings recognized by the model
	KeyMap KeyMap

	// Styles represent the Styles struct used to render the datepicker
	Styles Styles

	// Focused indicates the component which the end user is focused on
	Focused Focus

	// Selected indicates whether a date is Selected in the datepicker
	Selected bool
}

// New returns the Model of the datepicker
func New(initial time.Time) *DatePickerModel {
	return &DatePickerModel{
		Time:   initial,
		KeyMap: DefaultKeyMap(),
		Styles: DefaultStyles(),

		Focused:  FocusCalendar,
		Selected: false,
	}
}

// Init satisfies the `tea.Model` interface. This sends a nil cmd
func (m *DatePickerModel) Init() tea.Cmd {
	return nil
}

// Update changes the state of the datepicker. Update satisfies the `tea.Model` interface
func (m *DatePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.KeyMap.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.KeyMap.Up):
			m.updateUp()

		case key.Matches(msg, m.KeyMap.Right):
			m.updateRight()

		case key.Matches(msg, m.KeyMap.Down):
			m.updateDown()

		case key.Matches(msg, m.KeyMap.Left):
			m.updateLeft()

		case key.Matches(msg, m.KeyMap.FocusPrev):
			switch m.Focused {
			case FocusHeaderYear:
				m.SetFocus(FocusHeaderMonth)
			case FocusCalendar:
				m.SetFocus(FocusHeaderYear)
			case FocusNone, FocusHeaderMonth:
				// no previous field to move to
			}

		case key.Matches(msg, m.KeyMap.FocusNext):
			switch m.Focused {
			case FocusHeaderMonth:
				m.SetFocus(FocusHeaderYear)
			case FocusHeaderYear:
				m.SetFocus(FocusCalendar)
			case FocusNone, FocusCalendar:
				// no next field to move to
			}
		}
	}
	return m, nil
}

func (m *DatePickerModel) updateUp() {
	switch m.Focused {
	case FocusHeaderYear:
		m.LastYear()
	case FocusHeaderMonth:
		m.LastMonth()
	case FocusCalendar:
		m.LastWeek()
	case FocusNone:
		// do nothing
	}
}

func (m *DatePickerModel) updateRight() {
	switch m.Focused {
	case FocusHeaderYear:
		// do nothing
	case FocusHeaderMonth:
		m.SetFocus(FocusHeaderYear)
	case FocusCalendar:
		m.Tomorrow()
	case FocusNone:
		// do nothing
	}
}

func (m *DatePickerModel) updateDown() {
	switch m.Focused {
	case FocusHeaderYear:
		m.NextYear()
	case FocusHeaderMonth:
		m.NextMonth()
	case FocusCalendar:
		m.NextWeek()
	case FocusNone:
		// do nothing
	}
}

func (m *DatePickerModel) updateLeft() {
	switch m.Focused {
	case FocusHeaderYear:
		m.SetFocus(FocusHeaderMonth)
	case FocusHeaderMonth:
		// do nothing
	case FocusCalendar:
		m.Yesterday()
	case FocusNone:
		// do nothing
	}
}

// View renders a month view as a multiline string in the bubbletea application.
// View satisfies the `tea.Model` interface.
func (m *DatePickerModel) View() tea.View {
	b := strings.Builder{}
	month := m.Time.Month()
	year := m.Time.Year()

	tMonth, tYear := month.String(), strconv.Itoa(year)

	if m.Focused == FocusHeaderMonth {
		tMonth = m.Styles.FocusedText.Render(tMonth)
	} else {
		tMonth = m.Styles.HeaderText.Render(tMonth)
	}

	if m.Focused == FocusHeaderYear {
		tYear = m.Styles.FocusedText.Render(tYear)
	} else {
		tYear = m.Styles.HeaderText.Render(tYear)
	}

	title := m.Styles.Header.Render(fmt.Sprintf("%s %s\n", tMonth, tYear))

	// get all the dates of the current month
	firstDayOfTheMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	lastSundayOfLastMonth := firstDayOfTheMonth.AddDate(0, 0, -1)
	for lastSundayOfLastMonth.Weekday() != time.Sunday {
		lastSundayOfLastMonth = lastSundayOfLastMonth.AddDate(0, 0, -1)
	}

	lastDayOfTheMonth := firstDayOfTheMonth.AddDate(0, 1, -1)

	firstSundayOfNextMonth := lastDayOfTheMonth.AddDate(0, 0, 1)
	for firstSundayOfNextMonth.Weekday() != time.Sunday {
		firstSundayOfNextMonth = firstSundayOfNextMonth.AddDate(0, 0, 1)
	}

	day := lastSundayOfLastMonth
	if firstDayOfTheMonth.Weekday() == time.Sunday {
		day = firstDayOfTheMonth
	}

	weekHeaders := []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}
	for i, h := range weekHeaders {
		weekHeaders[i] = m.Styles.Date.Inherit(m.Styles.HeaderText).Render(h)
	}

	cal := [][]string{weekHeaders}
	j := 1

	for day.Before(firstSundayOfNextMonth) {
		if j >= len(cal) {
			cal = append(cal, []string{})
		}
		out := "  "
		if day.Month() == month {
			out = fmt.Sprintf("%02d", day.Day())
		}

		style := m.Styles.Date
		textStyle := m.Styles.Text
		switch {
		case !m.Selected:
			// skip modifications to the date
		case day.Day() == m.Time.Day() && day.Month() == m.Time.Month() && m.Focused == FocusCalendar:
			textStyle = m.Styles.FocusedText
		case day.Day() == m.Time.Day() && day.Month() == m.Time.Month():
			textStyle = m.Styles.SelectedText
		}

		out = style.Inherit(textStyle).Render(out)
		cal[j] = append(cal[j], out)

		if day.Weekday() == time.Saturday {
			j++
		}
		day = day.AddDate(0, 0, 1)
	}

	rows := make([]string, 0, 1+len(cal))
	rows = append(rows, title)
	for _, row := range cal {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Center, row...))
	}
	b.WriteString(lipgloss.JoinVertical(lipgloss.Center, rows...))

	return tea.NewView(b.String())
}

// SetsFocus focuses one of the datepicker components. This can also be used to blur
// the datepicker by passing the Focus `FocusNone`.
func (m *DatePickerModel) SetFocus(f Focus) {
	m.Focused = f
}

// Blur sets the datepicker focus to `FocusNone`
func (m *DatePickerModel) Blur() {
	m.Focused = FocusNone
}

// SetTime sets the model's `Time` struct and is used as reference to the selected date
func (m *DatePickerModel) SetTime(t time.Time) {
	m.Time = t
}

// LastWeek sets the model's `Time` struct back 7 days
func (m *DatePickerModel) LastWeek() {
	m.Time = m.Time.AddDate(0, 0, -7)
}

// NextWeek sets the model's `Time` struct forward 7 days
func (m *DatePickerModel) NextWeek() {
	m.Time = m.Time.AddDate(0, 0, 7)
}

// Yesterday sets the model's `Time` struct back 1 day
func (m *DatePickerModel) Yesterday() {
	m.Time = m.Time.AddDate(0, 0, -1)
}

// Tomorrow sets the model's `Time` struct forward 1 day
func (m *DatePickerModel) Tomorrow() {
	m.Time = m.Time.AddDate(0, 0, 1)
}

// LastMonth sets the model's `Time` struct back 1 month
func (m *DatePickerModel) LastMonth() {
	m.Time = m.Time.AddDate(0, -1, 0)
}

// NextMonth sets the model's `Time` struct forward 1 month
func (m *DatePickerModel) NextMonth() {
	m.Time = m.Time.AddDate(0, 1, 0)
}

// LastYear sets the model's `Time` struct back 1 year
func (m *DatePickerModel) LastYear() {
	m.Time = m.Time.AddDate(-1, 0, 0)
}

// NextYear sets the model's `Time` struct forward 1 year
func (m *DatePickerModel) NextYear() {
	m.Time = m.Time.AddDate(1, 0, 0)
}

// SelectDate changes the model's Selected to true
func (m *DatePickerModel) SelectDate() {
	m.Selected = true
}

// UnselectDate changes the model's Selected to false
func (m *DatePickerModel) UnselectDate() {
	m.Selected = false
}
