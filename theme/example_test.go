package theme_test

import (
	"fmt"

	"github.com/jarvisfriends/tui-base/theme"
)

// ExampleActive shows the one accessor every render path uses: the current
// AppStyle, never nil, reflecting the active tint/preset/mode/accessibility
// axes.
func ExampleActive() {
	c := theme.Active()
	fmt.Println(c != nil)
	// Output: true
}
