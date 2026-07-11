package inspector

import (
	"testing"

	"github.com/jarvisfriends/snap/rendercheck"
)

// TestInspectorBorderIntegrity asserts the CF-3 invariant on the inspector's
// bordered box across every standard terminal size: no line ever carries more
// (or fewer) than the two edge glyphs, which would mean inner content wrapped
// inside the border.
func TestInspectorBorderIntegrity(t *testing.T) {
	t.Parallel()
	rendercheck.CheckBorderIntegrity(t, New(), "│")
}
