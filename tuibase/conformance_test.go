package tuibase_test

import (
	"testing"

	"github.com/jarvisfriends/snap/rendercheck"
	"github.com/jarvisfriends/tui-base/testutil"
)

func TestCodeStandards(t *testing.T) {
	rendercheck.CheckCodeStandards(t, "github.com/jarvisfriends/tui-base/...")
	testutil.CheckDescriptiveStructNames(t, "github.com/jarvisfriends/tui-base/...")
}
