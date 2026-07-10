package inspector

import (
	"os"
	"testing"

	"github.com/jarvisfriends/snap/rendercheck"
	"github.com/jarvisfriends/tui-base/theme"
	tint "github.com/lrstanley/bubbletint/v2"
)

func TestMain(m *testing.M) {
	tint.NewDefaultRegistry()
	os.Exit(m.Run())
}

func TestInspectorLayoutOverflows(t *testing.T) {
	_ = theme.SetCurrentTint("dracula")
	m := New()
	m.SetColors(theme.Active())

	rendercheck.CheckNoLineOverflowAtSizes(t, m)
}
