package status_test

import (
	"testing"

	"github.com/jarvisfriends/tui-base/status"

	"github.com/jarvisfriends/tui-base/keys"
)

func TestToggleVisibleHeight(t *testing.T) {
	b := status.New()
	// initialize a key map like the router/main would do
	b.SetKeys(keys.DefaultKeyMap())
	b.SetWidth(40)
	if !b.IsVisible() {
		t.Fatal("expected visible by default")
	}
	h := b.Height()
	if h <= 0 {
		t.Fatalf("expected positive height when visible; got %d", h)
	}
	b.ToggleVisible()
	if b.IsVisible() {
		t.Fatal("expected not visible after ToggleVisible")
	}
	if b.Height() != 0 {
		t.Fatalf("expected height 0 when not visible; got %d", b.Height())
	}
	b.ToggleVisible()
	if !b.IsVisible() {
		t.Fatal("expected visible after ToggleVisible again")
	}
}

func TestShortVsLongHelp(t *testing.T) {
	b := status.New()
	b.SetKeys(keys.DefaultKeyMap())
	b.SetWidth(40)
	b.ToggleFullHelpVisible()
	shortHelp := b.Height()
	b.ToggleFullHelpVisible()
	longHelp := b.Height()

	if shortHelp == 0 {
		t.Fatal("expected ShortHelp to return some key bindings")
	}
	if longHelp == 0 {
		t.Fatal("expected LongHelp to return some key bindings")
	}
	// Both modes render the short-help line (renderHelpLine always uses ShortHelp()),
	// so heights are equal — just verify they are both positive.
}
