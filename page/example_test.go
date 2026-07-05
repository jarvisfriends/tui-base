package page_test

import (
	"fmt"

	"github.com/jarvisfriends/tui-base/page"
)

// ExampleBase_Colors shows the nil-safe theme accessor pages inherit by
// embedding Base: it returns a usable AppStyle even before the router has
// wired the shared theme pointer.
func ExampleBase_Colors() {
	var b page.Base
	fmt.Println(b.Colors() != nil)
	// Output: true
}
