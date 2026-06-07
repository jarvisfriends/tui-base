package geom

import "testing"

func TestRectContains(t *testing.T) {
	r := Rect{X: 10, Y: 5, W: 4, H: 3} // covers x 10..13, y 5..7
	cases := []struct {
		x, y int
		want bool
	}{
		{10, 5, true},   // top-left corner
		{13, 7, true},   // bottom-right inclusive cell
		{14, 7, false},  // right edge exclusive
		{13, 8, false},  // bottom edge exclusive
		{9, 5, false},   // left of
		{10, 4, false},  // above
		{12, 6, true},   // interior
		{-1, -1, false}, // far outside
	}
	for _, c := range cases {
		if got := r.Contains(c.x, c.y); got != c.want {
			t.Errorf("Rect%v.Contains(%d,%d) = %v, want %v", r, c.x, c.y, got, c.want)
		}
	}
}

func TestRectEmpty(t *testing.T) {
	for _, c := range []struct {
		r    Rect
		want bool
	}{
		{Rect{}, true},
		{Rect{W: 0, H: 5}, true},
		{Rect{W: 5, H: 0}, true},
		{Rect{W: 1, H: 1}, false},
	} {
		if got := c.r.Empty(); got != c.want {
			t.Errorf("Rect%v.Empty() = %v, want %v", c.r, got, c.want)
		}
	}
}

func TestRectCenteredIn(t *testing.T) {
	// A 20x4 box centered in 100x40 -> x=(100-20)/2=40, y=(40-4)/2=18.
	got := Rect{W: 20, H: 4}.CenteredIn(100, 40)
	if got != (Rect{X: 40, Y: 18, W: 20, H: 4}) {
		t.Fatalf("CenteredIn = %v, want {40 18 20 4}", got)
	}
	// A box larger than the area clamps the top-left to 0 (never negative).
	got = Rect{W: 200, H: 60}.CenteredIn(100, 40)
	if got.X != 0 || got.Y != 0 {
		t.Fatalf("CenteredIn oversize = %v, want X=0 Y=0", got)
	}
}
