// Package geom provides small screen-geometry primitives shared across the
// framework and its consumers. The charm v2 rendering model does mouse handling
// through the tea.View OnMouse callback with explicit rectangle hit-testing
// (rather than a zone library), so a single Rect type keeps that logic
// consistent everywhere: router overlays, page edit-overlays, and widget grids.
package geom

// Rect is an axis-aligned rectangle in terminal cells: top-left (X, Y) with
// width W and height H.
type Rect struct{ X, Y, W, H int }

// Contains reports whether the cell (x, y) falls inside the rectangle. The right
// and bottom edges are exclusive (a W×H rect covers columns X..X+W-1).
func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Empty reports whether the rectangle has no area.
func (r Rect) Empty() bool { return r.W <= 0 || r.H <= 0 }

// CenteredIn returns a copy of r positioned so its W×H box is centered within an
// areaW×areaH region, clamped so the top-left never goes negative. Width and
// height are preserved. Use it to place centered overlays without repeating the
// max(0, (area-size)/2) arithmetic.
func (r Rect) CenteredIn(areaW, areaH int) Rect {
	r.X = max(0, (areaW-r.W)/2)
	r.Y = max(0, (areaH-r.H)/2)
	return r
}
