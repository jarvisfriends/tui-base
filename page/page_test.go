package page

import (
	"testing"

	"github.com/jarvisfriends/tui-base/theme"
)

// TestBaseColorsFallback verifies Colors() returns the wired palette when set
// and falls back to theme.Active() (never nil) otherwise.
func TestBaseColorsFallback(t *testing.T) {
	var b Base

	// Before SetColors: must fall back to the active palette, never nil.
	if got := b.Colors(); got == nil {
		t.Fatal("Colors() returned nil before SetColors; expected theme.Active() fallback")
	}

	want := theme.FromTint(nil) // a concrete, distinct palette pointer
	b.SetColors(want)
	if got := b.Colors(); got != want {
		t.Fatalf("Colors() = %p after SetColors(%p); want the wired pointer", got, want)
	}
}

// TestBaseSize verifies SetSize round-trips through Width/Height.
func TestBaseSize(t *testing.T) {
	var b Base
	if b.Width() != 0 || b.Height() != 0 {
		t.Fatalf("zero-value size = %dx%d; want 0x0", b.Width(), b.Height())
	}
	b.SetSize(120, 40)
	if b.Width() != 120 || b.Height() != 40 {
		t.Fatalf("after SetSize(120,40) size = %dx%d", b.Width(), b.Height())
	}
}

// TestBaseSatisfiesColorAware ensures an embedder of Base satisfies the
// theme.ColorAware interface that the router uses to broadcast palette updates.
func TestBaseSatisfiesColorAware(t *testing.T) {
	type page struct{ Base }
	var _ theme.ColorAware = (*page)(nil)

	p := &page{}
	c := theme.FromTint(nil)
	p.SetColors(c) // promoted from Base
	if p.Colors() != c {
		t.Fatal("embedded Base did not store colors via promoted SetColors")
	}
}
