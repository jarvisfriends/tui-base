package settings_test

import (
	"testing"

	"github.com/jarvisfriends/tui-base/pages/settings"
	"github.com/jarvisfriends/tui-base/testutil"
	"github.com/jarvisfriends/tui-base/theme"
	tint "github.com/lrstanley/bubbletint/v2"
)

func init() {
	tint.NewDefaultRegistry()
}

func TestSettingsLayoutOverflows(t *testing.T) {
	_ = theme.SetCurrentTint("dracula")
	m := settings.New()
	m.SetColors(theme.Active())

	testutil.CheckNoLineOverflowAtSizes(t, m)
}

func TestSettingsNarrowWidths(t *testing.T) {
	_ = theme.SetCurrentTint("dracula")
	m := settings.New()
	m.SetColors(theme.Active())

	// Check narrow width rendering down to min width (30 columns)
	testutil.CheckNoBorderOverflow(t, m, 30, 24)
}
