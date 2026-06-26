package theme

import (
	"image/color"
	"math"

	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"
	"github.com/lucasb-eyer/go-colorful"
)

// ColorPair represents a foreground/background color combination with a name.
type ColorPair struct {
	Name string
	Fg   color.Color
	Bg   color.Color
}

// cvdMatrices hold the transformation matrices for simulating color vision deficiencies.
var cvdMatrices = [...][3][3]float64{
	{ // Protanopia (red blindness)
		{0.56667, 0.43333, 0},
		{0.55833, 0.44167, 0},
		{0, 0.24167, 0.75833},
	},
	{ // Deuteranopia (green blindness)
		{0.625, 0.375, 0},
		{0.7, 0.3, 0},
		{0, 0.3, 0.7},
	},
	{ // Tritanopia (blue-yellow blindness)
		{0.95, 0.05, 0},
		{0, 0.43333, 0.56667},
		{0, 0.475, 0.525},
	},
}

// colorPairsFromSimple generates color pairs for the fallback dark-terminal palette.
func colorPairsFromSimple() []ColorPair {
	bg := lipgloss.Color("235")
	return []ColorPair{
		{Name: "Black", Fg: lipgloss.Color("16"), Bg: bg},
		{Name: "Red", Fg: lipgloss.Color("1"), Bg: bg},
		{Name: "Green", Fg: lipgloss.Color("2"), Bg: bg},
		{Name: "Yellow", Fg: lipgloss.Color("3"), Bg: bg},
		{Name: "Blue", Fg: lipgloss.Color("4"), Bg: bg},
		{Name: "Magenta", Fg: lipgloss.Color("5"), Bg: bg},
		{Name: "Cyan", Fg: lipgloss.Color("6"), Bg: bg},
		{Name: "White", Fg: lipgloss.Color("7"), Bg: bg},
		{Name: "Bright Black", Fg: lipgloss.Color("240"), Bg: bg},
		{Name: "Bright Red", Fg: lipgloss.Color("9"), Bg: bg},
		{Name: "Bright Green", Fg: lipgloss.Color("10"), Bg: bg},
		{Name: "Bright Yellow", Fg: lipgloss.Color("11"), Bg: bg},
		{Name: "Bright Blue", Fg: lipgloss.Color("12"), Bg: bg},
		{Name: "Bright Magenta", Fg: lipgloss.Color("13"), Bg: bg},
		{Name: "Bright Cyan", Fg: lipgloss.Color("14"), Bg: bg},
		{Name: "Bright White", Fg: lipgloss.Color("15"), Bg: bg},
	}
}

// colorPairsFromTint generates color pairs from a bubbletint Tint.
// If adjustForAccess is true, colors are adjusted to improve accessibility.
func colorPairsFromTint(t *tint.Tint, adjustForAccess bool) []ColorPair {
	if t == nil {
		return colorPairsFromSimple()
	}

	var pairs []ColorPair
	if t.Bg != nil {
		bg := lipgloss.Color(t.Bg.Hex())
		pairs = append(
			pairs,
			ColorPair{Name: "Black", Fg: col(t.Black, "16"), Bg: bg},
			ColorPair{Name: "Red", Fg: col(t.Red, "1"), Bg: bg},
			ColorPair{Name: "Green", Fg: col(t.Green, "2"), Bg: bg},
			ColorPair{Name: "Yellow", Fg: col(t.Yellow, "3"), Bg: bg},
			ColorPair{Name: "Blue", Fg: col(t.Blue, "4"), Bg: bg},
			ColorPair{Name: "Purple", Fg: col(t.Purple, "5"), Bg: bg},
			ColorPair{Name: "Cyan", Fg: col(t.Cyan, "6"), Bg: bg},
			ColorPair{Name: "White", Fg: col(t.White, "7"), Bg: bg},
			ColorPair{Name: "Bright Black", Fg: col(t.BrightBlack, "240"), Bg: bg},
			ColorPair{Name: "Bright Red", Fg: col(t.BrightRed, "9"), Bg: bg},
			ColorPair{Name: "Bright Green", Fg: col(t.BrightGreen, "10"), Bg: bg},
			ColorPair{Name: "Bright Yellow", Fg: col(t.BrightYellow, "11"), Bg: bg},
			ColorPair{Name: "Bright Blue", Fg: col(t.BrightBlue, "12"), Bg: bg},
			ColorPair{Name: "Bright Purple", Fg: col(t.BrightPurple, "13"), Bg: bg},
			ColorPair{Name: "Bright Cyan", Fg: col(t.BrightCyan, "14"), Bg: bg},
			ColorPair{Name: "Bright White", Fg: col(t.BrightWhite, "15"), Bg: bg},
		)
	}
	if t.SelectionBg != nil {
		bg := lipgloss.Color(t.SelectionBg.Hex())
		pairs = append(
			pairs,
			ColorPair{Name: "Select Black", Fg: col(t.Black, "16"), Bg: bg},
			ColorPair{Name: "Select Red", Fg: col(t.Red, "1"), Bg: bg},
			ColorPair{Name: "Select Green", Fg: col(t.Green, "2"), Bg: bg},
			ColorPair{Name: "Select Yellow", Fg: col(t.Yellow, "3"), Bg: bg},
			ColorPair{Name: "Select Blue", Fg: col(t.Blue, "4"), Bg: bg},
			ColorPair{Name: "Select Purple", Fg: col(t.Purple, "5"), Bg: bg},
			ColorPair{Name: "Select Cyan", Fg: col(t.Cyan, "6"), Bg: bg},
			ColorPair{Name: "Select White", Fg: col(t.White, "7"), Bg: bg},
			ColorPair{Name: "Select Bright Black", Fg: col(t.BrightBlack, "240"), Bg: bg},
			ColorPair{Name: "Select Bright Red", Fg: col(t.BrightRed, "9"), Bg: bg},
			ColorPair{Name: "Select Bright Green", Fg: col(t.BrightGreen, "10"), Bg: bg},
			ColorPair{Name: "Select Bright Yellow", Fg: col(t.BrightYellow, "11"), Bg: bg},
			ColorPair{Name: "Select Bright Blue", Fg: col(t.BrightBlue, "12"), Bg: bg},
			ColorPair{Name: "Select Bright Purple", Fg: col(t.BrightPurple, "13"), Bg: bg},
			ColorPair{Name: "Select Bright Cyan", Fg: col(t.BrightCyan, "14"), Bg: bg},
			ColorPair{Name: "Select Bright White", Fg: col(t.BrightWhite, "15"), Bg: bg},
		)
	}

	if adjustForAccess {
		for i, p := range pairs {
			if adjusted := tryAdjustForAccess(p.Fg, p.Bg); adjusted != nil {
				pairs[i].Fg = adjusted
			}
		}
	}

	return pairs
}

// AccessiblePairsFromTint returns accessibility-adjusted color pairs for a tint.
// Use this in diagnostics UIs; avoid in hot render paths.
func AccessiblePairsFromTint(t *tint.Tint) []ColorPair {
	return colorPairsFromTint(t, true)
}

// CVD helpers for adjusting colors for accessibility.
func cvdLuminance(c colorful.Color) float64 {
	lin := func(v float64) float64 {
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

func cvdContrast(fg, bg colorful.Color) float64 {
	lf, lb := cvdLuminance(fg), cvdLuminance(bg)
	if lf < lb {
		lf, lb = lb, lf
	}
	return (lf + 0.05) / (lb + 0.05)
}

func cvdApply(c colorful.Color, matrix [3][3]float64) colorful.Color {
	return colorful.Color{
		R: matrix[0][0]*c.R + matrix[0][1]*c.G + matrix[0][2]*c.B,
		G: matrix[1][0]*c.R + matrix[1][1]*c.G + matrix[1][2]*c.B,
		B: matrix[2][0]*c.R + matrix[2][1]*c.G + matrix[2][2]*c.B,
	}.Clamped()
}

// tryAdjustForAccess attempts to make a foreground color more accessible against its background.
// Returns the adjusted color.Color, or nil if adjustment is not needed or not possible.
func tryAdjustForAccess(fgColor, bgColor color.Color) color.Color {
	fgC, ok := colorful.MakeColor(fgColor)
	if !ok {
		return nil
	}
	bgC, ok := colorful.MakeColor(bgColor)
	if !ok {
		return nil
	}

	minContrast := 3.0
	minCVDistance := 0.05
	minCVContrast := 2.5

	// Check if already accessible.
	normalContrast := cvdContrast(fgC, bgC)
	if normalContrast < minContrast {
		// Need adjustment.
		suggested := suggestAccessibleForeground(fgC, bgC, minContrast, minCVDistance, minCVContrast)
		if suggested != nil && !almostEqualColor(*suggested, fgC) {
			return lipgloss.Color(suggested.Hex())
		}
	}

	return nil
}

func suggestAccessibleForeground(fg, bg colorful.Color, minContrast, minCVDist, minCVContrast float64) *colorful.Color {
	step := 0.02
	targets := []colorful.Color{{R: 0, G: 0, B: 0}, {R: 1, G: 1, B: 1}}
	bestPassing := colorful.Color{}
	bestDist := math.MaxFloat64

	for _, target := range targets {
		for blend := 0.0; blend <= 1.0; blend += step {
			candidate := fg.BlendLab(target, blend).Clamped()
			if meetsAccessibilityThreshold(candidate, bg, minContrast, minCVDist, minCVContrast) {
				dist := fg.DistanceCIEDE2000(candidate)
				if dist < bestDist {
					bestPassing = candidate
					bestDist = dist
				}
			}
		}
	}

	if bestDist < math.MaxFloat64 {
		return &bestPassing
	}
	return nil
}

func meetsAccessibilityThreshold(fg, bg colorful.Color, minContrast, minCVDist, minCVContrast float64) bool {
	if cvdContrast(fg, bg) < minContrast {
		return false
	}

	for _, matrix := range cvdMatrices {
		sfg := cvdApply(fg, matrix)
		sbg := cvdApply(bg, matrix)
		if sfg.DistanceCIEDE2000(sbg) < minCVDist {
			return false
		}
		if cvdContrast(sfg, sbg) < minCVContrast {
			return false
		}
	}
	return true
}

func almostEqualColor(a, b colorful.Color) bool {
	const eps = 1e-12
	return math.Abs(a.R-b.R) < eps && math.Abs(a.G-b.G) < eps && math.Abs(a.B-b.B) < eps
}

// applyAccessibilityAdjustments mutates a palette's semantic foreground colors so
// each meets contrast and color-vision thresholds against its paired background.
func applyAccessibilityAdjustments(colors *AppStyle) {
	if colors == nil {
		return
	}
	adjust := func(fg *color.Color, bg color.Color) {
		if fg == nil {
			return
		}
		if adjusted := tryAdjustForAccess(*fg, bg); adjusted != nil {
			*fg = adjusted
		}
	}

	adjust(&colors.Fg, colors.Bg)
	adjust(&colors.Muted, colors.Bg)
	adjust(&colors.Border, colors.Bg)
	adjust(&colors.Accent, colors.Bg)
	adjust(&colors.SelectionFg, colors.SelectionBg)
	adjust(&colors.StatusFg, colors.StatusBg)
	adjust(&colors.Success, colors.Bg)
	adjust(&colors.Error, colors.Bg)
	adjust(&colors.Warning, colors.Bg)
}
