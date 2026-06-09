package inspector

import (
	"testing"

	"github.com/jarvisfriends/tui-base/testutil"
	"github.com/jarvisfriends/tui-base/theme"
	tint "github.com/lrstanley/bubbletint/v2"
)

func init() {
	tint.NewDefaultRegistry()
}

func TestInspectorLayoutOverflows(t *testing.T) {
	_ = theme.SetCurrentTint("dracula")
	m := New()
	m.SetColors(theme.Active())

	testutil.CheckNoLineOverflowAtSizes(t, m)
}
