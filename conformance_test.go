package main_test

import (
	"testing"

	"github.com/jarvisfriends/tui-base/testutil"
)

func TestCodeStandards(t *testing.T) {
	testutil.CheckCodeStandards(t, "github.com/jarvisfriends/tui-base/...")
}
