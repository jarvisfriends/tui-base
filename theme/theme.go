package theme

import (
	"image/color"
	"math"
	"strconv"
	"strings"
	"sync"

	"charm.land/bubbles/v2/help"
	key "charm.land/bubbles/v2/key"
	"charm.land/huh/v2"
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

// cvdMatrices hold the transformation matrices for simulating color vision deficiencies
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

var (
	huhThemeCacheMu sync.Mutex
	huhThemeCacheID string
	huhThemeCache   *huh.Styles

	appStyleCacheMu sync.RWMutex
	appStyleCache   = map[string]*AppStyle{}

	themePrefsMu sync.RWMutex
	themePrefs   = ThemePreferences{Mode: ThemeModeDark}
)

const (
	ThemeModeDark  = "dark"
	ThemeModeLight = "light"
)

// ThemePreferences controls global theme behavior applied by Active.
type ThemePreferences struct {
	Mode          string
	Accessibility bool
}

// StyleCombo describes one concrete foreground/background pair used by the UI.
// It is primarily used for diagnostics and temporary accessibility tests.
type StyleCombo struct {
	Name string
	Fg   color.Color
	Bg   color.Color
}

// AppStyle holds the semantic color palette for the application, derived from
// the active bubbletint theme. Each field maps a UI role to a color.Color.
// Call [Active] on every render to pick up live theme changes.
type AppStyle struct {
	// Fg is the primary foreground / body text color.
	Fg color.Color
	// Bg is the primary background color.
	Bg color.Color
	// Muted is used for secondary / dimmed text and inactive navigation items
	// (maps to the "comment" or "bright_black" slot in most terminal themes).
	Muted color.Color
	// Border is used for borders and dividers (typically slightly darker than Muted).
	Border color.Color
	// Accent is the primary accent color: navigation titles, box titles, and
	// tab / form highlights (maps to the purple/violet slot).
	Accent color.Color
	// SelectionBg is the background of the active / selected navigation item.
	SelectionBg color.Color
	// SelectionFg is the foreground of the selected navigation item.
	SelectionFg color.Color
	// StatusBg is the status bar background color.
	StatusBg color.Color
	// StatusFg is the status bar foreground color.
	StatusFg color.Color
	// Success is used for affirmative / selected-option states (green slot).
	Success color.Color
	// Error is used for error indicators (red slot).
	Error color.Color
	// Warning is used for warning / indicator cues (yellow slot).
	Warning color.Color

	HuhStyle        *huh.Styles //
	Styles          *Styles     // pre-computed lipgloss styles for this palette
	OrigTint        *tint.Tint  // the original tint this palette was derived from; used for debugging and testing
	AccessibleTint  *tint.Tint  // a suggested tint with improved accessibility, if the original fails; used for debugging and testing
	OrigPairs       []ColorPair // all color combinations from the original tint
	AccessiblePairs []ColorPair // the same pairs but with colors adjusted for accessibility where needed
}

// Styles holds pre-computed lipgloss styles derived from a ThemeColors palette.
// Styles are rebuilt when the theme changes or the terminal background is detected.
type Styles struct {
	Name string

	Help help.Styles

	Input  lipgloss.Style // TextInputStyles
	Cursor lipgloss.Style // TextInputStyles.Cursor

	// Pre-computed styles — use these instead of calling lipgloss.NewStyle() inline.
	Title      lipgloss.Style
	Subtitle   lipgloss.Style
	RealHeader lipgloss.Style
	TextOnBg   lipgloss.Style
	BoldText   lipgloss.Style
	Dim        lipgloss.Style

	BoarderActive   lipgloss.Style
	BoarderInactive lipgloss.Style

	Item         lipgloss.Style
	SelectedItem lipgloss.Style

	Send lipgloss.Style
	Wait lipgloss.Style

	Online  lipgloss.Style //
	Offline lipgloss.Style

	FilterActive lipgloss.Style
	FilterDim    lipgloss.Style

	StatusBase    lipgloss.Style
	StatusKey     lipgloss.Style
	StatusKeyBold lipgloss.Style
	StatusDesc    lipgloss.Style
	StatusSep     lipgloss.Style

	OverlayBorder lipgloss.Style
	OverlayPanel  lipgloss.Style

	NavTitle     lipgloss.Style
	NavActive    lipgloss.Style
	NavInactive  lipgloss.Style
	NavContainer lipgloss.Style

	TabInactive lipgloss.Style
	TabHover    lipgloss.Style

	SwatchDot lipgloss.Style
	Row       lipgloss.Style

	Success lipgloss.Style // tint.Green
	Error   lipgloss.Style // tint.Red("Error")
	Warning lipgloss.Style // tint.Yellow("Warning")

	// Pre-rendered strings
	SuccessMark string // "✓" in SuccessColor
	ErrorMark   string // "✗" in ErrorColor
	Gap         string // single space on PageBg
}

// col returns a color.Color from a *tint.Color, using fallback ANSI/hex string when nil.
func col(c *tint.Color, fallback string) color.Color {
	if c != nil {
		return lipgloss.Color(c.Hex())
	}
	return lipgloss.Color(fallback)
}

// colorPairsFromSimple generates color pairs from simple ANSI color codes.
// Used as a fallback when no theme is available.
func colorPairsFromSimple(fgCode, bgCode string) []ColorPair {
	bg := lipgloss.Color(bgCode)
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
		return colorPairsFromSimple("250", "235")
	}

	var pairs []ColorPair
	if t.Bg != nil {
		bg := lipgloss.Color(t.Bg.Hex())
		pairs = append(pairs,
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
		pairs = append(pairs,
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

// CVD helpers for adjusting colors for accessibility
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

	// Check if already accessible
	normalContrast := cvdContrast(fgC, bgC)
	if normalContrast < minContrast {
		// Need adjustment
		suggested := suggestAccessibleForeground(fgC, bgC, minContrast, minCVDistance, minCVContrast)
		if suggested != nil && !almostEqualColor(*suggested, fgC) {
			return lipgloss.Color((*suggested).Hex())
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

// NormalizeMode returns the normalized theme mode value.
func NormalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ThemeModeLight:
		return ThemeModeLight
	default:
		return ThemeModeDark
	}
}

// SetThemePreferences updates global preferences used by Active.
func SetThemePreferences(mode string, accessibility bool) {
	themePrefsMu.Lock()
	themePrefs.Mode = NormalizeMode(mode)
	themePrefs.Accessibility = accessibility
	themePrefsMu.Unlock()
}

// ThemePreferencesSnapshot returns a copy of the current global preferences.
func ThemePreferencesSnapshot() ThemePreferences {
	themePrefsMu.RLock()
	defer themePrefsMu.RUnlock()
	return themePrefs
}

// ResolveTintIDForMode returns a tint ID matching the requested mode.
// If requestedID already matches, it is returned unchanged.
func ResolveTintIDForMode(requestedID string, mode string) string {
	requestedModeDark := NormalizeMode(mode) == ThemeModeDark
	tints := tint.Tints()
	if len(tints) == 0 {
		return requestedID
	}

	if requestedID != "" {
		for _, t := range tints {
			if t.ID == requestedID && t.Dark == requestedModeDark {
				return requestedID
			}
		}
	}

	for _, t := range tints {
		if t.Dark == requestedModeDark {
			return t.ID
		}
	}

	if requestedID != "" {
		return requestedID
	}
	return tints[0].ID
}

func tintForMode(current *tint.Tint, mode string) *tint.Tint {
	if current == nil {
		return nil
	}
	requestedModeDark := NormalizeMode(mode) == ThemeModeDark
	if current.Dark == requestedModeDark {
		return current
	}
	resolved := ResolveTintIDForMode(current.ID, mode)
	for _, candidate := range tint.Tints() {
		if candidate.ID == resolved {
			return candidate
		}
	}
	return current
}

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

// Active returns the current AppColors palette derived from the active bubbletint.
// It is safe to call before the registry has been initialised; a built-in fallback
// palette (matching the Dracula aesthetic) is returned in that case.
func Active() *AppStyle {
	prefs := ThemePreferencesSnapshot()
	var t *tint.Tint
	func() {
		defer func() { recover() }() //nolint:errcheck
		t = tint.Current()
	}()
	t = tintForMode(t, prefs.Mode)
	return FromTintWithOptions(t, prefs.Accessibility)
}

// FromTint maps a *tint.Tint onto the application's semantic AppColors.
// Every field has a hardcoded fallback that works in any 256-color terminal.
func FromTint(t *tint.Tint) *AppStyle {
	return FromTintWithOptions(t, false)
}

// FromTintWithOptions maps a tint into AppStyle with optional accessibility
// adjustments for semantic foreground/background pairs.
func FromTintWithOptions(t *tint.Tint, accessibility bool) *AppStyle {
	cacheKey := "fallback"
	if t != nil && t.ID != "" {
		cacheKey = t.ID
	}
	if accessibility {
		cacheKey += "|access"
	}

	appStyleCacheMu.RLock()
	cached, ok := appStyleCache[cacheKey]
	appStyleCacheMu.RUnlock()
	if ok {
		return cached
	}

	if t == nil {
		pairs := colorPairsFromSimple("250", "235")
		colors := &AppStyle{
			Fg:              lipgloss.Color("250"),
			Bg:              lipgloss.Color("235"),
			Muted:           lipgloss.Color("240"),
			Border:          lipgloss.Color("238"),
			Accent:          lipgloss.Color("205"),
			SelectionBg:     lipgloss.Color("62"),
			SelectionFg:     lipgloss.Color("255"),
			StatusBg:        lipgloss.Color("236"),
			StatusFg:        lipgloss.Color("250"),
			Success:         lipgloss.Color("35"),
			Error:           lipgloss.Color("9"),
			Warning:         lipgloss.Color("11"),
			OrigPairs:       pairs,
			AccessiblePairs: colorPairsFromSimple("250", "235"),
		}
		if accessibility {
			applyAccessibilityAdjustments(colors)
			colors.AccessiblePairs = colorPairsFromSimple("250", "235")
		}
		colors.Styles = BuildStyles(colors)

		appStyleCacheMu.Lock()
		appStyleCache[cacheKey] = colors
		appStyleCacheMu.Unlock()
		return colors
	}

	// Selection background: prefer the theme's explicit selection color, then
	// fall back to its blue slot, then a reasonable default.
	var sel color.Color
	if t.SelectionBg != nil {
		sel = lipgloss.Color(t.SelectionBg.Hex())
	} else if t.Blue != nil {
		sel = lipgloss.Color(t.Blue.Hex())
	} else {
		sel = lipgloss.Color("62")
	}

	o := colorPairsFromTint(t, false)
	colors := &AppStyle{
		Fg:          col(t.Fg, "250"),
		Bg:          col(t.Bg, "235"),
		Muted:       col(t.BrightBlack, "240"),
		Border:      col(t.Black, "238"),
		Accent:      col(t.Purple, "205"),
		SelectionBg: sel,
		SelectionFg: col(t.BrightWhite, "255"),
		StatusBg:    col(t.Black, "236"),
		StatusFg:    col(t.Fg, "250"),
		Success:     col(t.Green, "35"),
		Error:       col(t.Red, "9"),
		Warning:     col(t.Yellow, "11"),
		OrigTint:    t,
		OrigPairs:   o,
		// Accessibility-adjusted pairs are expensive to compute; defer to
		// explicit callers (for example, the debug accessibility panel).
		AccessiblePairs: o,
	}
	if accessibility {
		applyAccessibilityAdjustments(colors)
		colors.AccessiblePairs = colorPairsFromTint(t, true)
	}
	colors.Styles = BuildStyles(colors)

	appStyleCacheMu.Lock()
	appStyleCache[cacheKey] = colors
	appStyleCacheMu.Unlock()
	return colors
}

// AccessiblePairsFromTint returns accessibility-adjusted color pairs for a tint.
// Use this in diagnostics UIs; avoid in hot render paths.
func AccessiblePairsFromTint(t *tint.Tint) []ColorPair {
	return colorPairsFromTint(t, true)
}

// StyleCombosFromAppStyle returns concrete fg/bg combinations from named styles.
func StyleCombosFromAppStyle(c *AppStyle) []StyleCombo {
	if c == nil || c.Styles == nil {
		return nil
	}
	combos := []StyleCombo{
		{Name: "Title", Fg: c.Styles.Title.GetForeground(), Bg: c.Styles.Title.GetBackground()},
		{Name: "Subtitle", Fg: c.Styles.Subtitle.GetForeground(), Bg: c.Styles.Subtitle.GetBackground()},
		{Name: "TextOnBg", Fg: c.Styles.TextOnBg.GetForeground(), Bg: c.Styles.TextOnBg.GetBackground()},
		{Name: "Dim", Fg: c.Styles.Dim.GetForeground(), Bg: c.Styles.Dim.GetBackground()},
		{Name: "SelectedItem", Fg: c.Styles.SelectedItem.GetForeground(), Bg: c.Styles.SelectedItem.GetBackground()},
		{Name: "StatusBase", Fg: c.Styles.StatusBase.GetForeground(), Bg: c.Styles.StatusBase.GetBackground()},
		{Name: "StatusKey", Fg: c.Styles.StatusKey.GetForeground(), Bg: c.Styles.StatusKey.GetBackground()},
		{Name: "StatusDesc", Fg: c.Styles.StatusDesc.GetForeground(), Bg: c.Styles.StatusDesc.GetBackground()},
		{Name: "NavActive", Fg: c.Styles.NavActive.GetForeground(), Bg: c.Styles.NavActive.GetBackground()},
		{Name: "NavInactive", Fg: c.Styles.NavInactive.GetForeground(), Bg: c.Styles.NavInactive.GetBackground()},
		{Name: "Send", Fg: c.Styles.Send.GetForeground(), Bg: c.Styles.Send.GetBackground()},
		{Name: "Success", Fg: c.Styles.Success.GetForeground(), Bg: c.Styles.Success.GetBackground()},
		{Name: "Error", Fg: c.Styles.Error.GetForeground(), Bg: c.Styles.Error.GetBackground()},
		{Name: "Warning", Fg: c.Styles.Warning.GetForeground(), Bg: c.Styles.Warning.GetBackground()},
	}
	out := make([]StyleCombo, 0, len(combos))
	for _, combo := range combos {
		if combo.Fg != nil && combo.Bg != nil {
			out = append(out, combo)
		}
	}
	return out
}

// BuildStyles pre-computes commonly used lipgloss styles from one palette.
func BuildStyles(c *AppStyle) *Styles {
	name := "active"
	if c.OrigTint != nil {
		name = c.OrigTint.DisplayName
	}

	base := lipgloss.NewStyle().Background(c.Bg).Foreground(c.Fg)
	statusBase := lipgloss.NewStyle().Background(c.StatusBg).Foreground(c.StatusFg)

	return &Styles{
		Name: name,

		Input:  base,
		Cursor: base.Foreground(c.Warning),

		Title:      base.Bold(true).Foreground(c.Accent),
		Subtitle:   base.Foreground(c.Muted),
		RealHeader: base.Bold(true).Foreground(c.Accent),
		TextOnBg:   base,
		BoldText:   base.Bold(true),
		Dim:        base.Faint(true),

		BoarderActive:   base.BorderForeground(c.Accent),
		BoarderInactive: base.BorderForeground(c.Border),

		Item:         base.Foreground(c.Muted),
		SelectedItem: base.Background(c.SelectionBg).Foreground(c.SelectionFg).Bold(true),

		Send: base.Background(c.SelectionBg).Foreground(c.SelectionFg).Padding(0, 1),
		Wait: base.Foreground(c.Muted),

		Online:  base.Foreground(c.Success),
		Offline: base.Foreground(c.Muted),

		FilterActive: base.Foreground(c.Accent).Bold(true),
		FilterDim:    base.Foreground(c.Muted),

		StatusBase:    statusBase,
		StatusKey:     statusBase.Foreground(c.Fg),
		StatusKeyBold: statusBase.Foreground(c.Fg).Bold(true),
		StatusDesc:    statusBase.Foreground(c.Muted),
		StatusSep:     statusBase.Foreground(c.Muted),

		OverlayBorder: base.Border(lipgloss.RoundedBorder()).BorderForeground(c.Accent),
		OverlayPanel:  base,

		NavTitle:     base.Bold(true).Foreground(c.Accent),
		NavActive:    base.Background(c.SelectionBg).Foreground(c.SelectionFg).Bold(true),
		NavInactive:  base.Foreground(c.Muted),
		NavContainer: base.BorderForeground(c.Border),

		TabInactive: base.BorderForeground(c.Accent),
		TabHover:    base.BorderForeground(c.Muted).Background(c.Border),

		SwatchDot: base,
		Row:       base,

		Success: base.Foreground(c.Success),
		Error:   base.Foreground(c.Error),
		Warning: base.Foreground(c.Warning),

		SuccessMark: base.Foreground(c.Success).Render("✓"),
		ErrorMark:   base.Foreground(c.Error).Render("✗"),
		Gap:         base.Render(" "),
		Help: help.Styles{
			Ellipsis:       statusBase.Foreground(c.Muted),
			ShortKey:       statusBase.Foreground(c.Accent),
			ShortDesc:      statusBase.Foreground(c.Fg),
			ShortSeparator: statusBase.Foreground(c.Muted),
			FullKey:        statusBase.Foreground(c.Accent),
			FullDesc:       statusBase.Foreground(c.Fg),
			FullSeparator:  statusBase.Foreground(c.Muted),
		},
	}
}

// BuildHuhStyles maps the active app palette onto huh styles.
func BuildHuhStyles(c *AppStyle) *huh.Styles {
	t := huh.ThemeBase(false)

	t.Focused.Base = t.Focused.Base.BorderForeground(c.Accent)
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = t.Focused.Title.Foreground(c.Accent).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(c.Accent).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(c.Muted)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(c.Error)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(c.Error)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(c.Warning)
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(c.Warning)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(c.Warning)
	t.Focused.Option = t.Focused.Option.Foreground(c.Fg)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(c.Success)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(c.Success)
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(c.Fg)
	t.Focused.UnselectedPrefix = t.Focused.UnselectedPrefix.Foreground(c.Muted)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(c.Bg).Background(c.Accent).Bold(true)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(c.Fg).Background(c.Border)
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(c.Warning)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(c.Muted)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(c.Warning)

	t.Blurred = t.Focused
	t.Blurred.Base = t.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.NextIndicator = c.Styles.TextOnBg
	t.Blurred.PrevIndicator = c.Styles.TextOnBg
	t.Blurred.Title = t.Focused.Title.Foreground(c.Muted).Bold(false)
	t.Blurred.Description = c.Styles.Dim
	t.Blurred.SelectedOption = t.Focused.SelectedOption.Foreground(c.Muted)
	t.Blurred.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(c.Muted)

	t.Group.Title = t.Focused.Title
	t.Group.Description = t.Focused.Description
	return t
}

// HuhThemeFunc returns a ThemeFunc that always maps from the latest Active palette.
func HuhThemeFunc() huh.ThemeFunc {
	return func(_ bool) *huh.Styles {
		active := Active()
		id := "fallback"
		if active.OrigTint != nil {
			id = active.OrigTint.ID
		}
		if ThemePreferencesSnapshot().Accessibility {
			id += "|access"
		}

		huhThemeCacheMu.Lock()
		defer huhThemeCacheMu.Unlock()

		if huhThemeCache != nil && huhThemeCacheID == id {
			copy := *huhThemeCache
			return &copy
		}

		huhThemeCache = BuildHuhStyles(active)
		huhThemeCacheID = id
		copy := *huhThemeCache
		return &copy
	}
}

// BoxStyle returns a rounded-border box style using the current theme colors.
func BoxStyle() lipgloss.Style {
	c := Active()
	return c.Styles.OverlayBorder.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c.Muted).
		Padding(1, 2)
}

// BoxTitleStyle returns a bold title style using the current accent color.
func BoxTitleStyle() lipgloss.Style {
	return Active().Styles.Title
}

// SubtleStyle returns a dimmed text style for secondary / hint content.
func SubtleStyle() lipgloss.Style {
	return Active().Styles.Subtitle
}

// RenderStatusBar composes a left-aligned help string and a right-aligned
// status string into a single styled bar of the given width. If width <= 0
// the function will return a simple un-padded rendering.
func RenderStatusBar(width int, left string, right string) string {
	return RenderStatusBarStyled(width, left, right, -1)
}

// RenderStatusBarStyled renders the status bar. When colorIndex >= 0 it
// overrides the foreground with that ANSI index (0-255), which is useful for
// fade-in/out animations. Pass -1 to use the theme's StatusFg color.
func RenderStatusBarStyled(width int, left string, right string, colorIndex int) string {
	c := Active()
	fg := c.StatusFg
	if colorIndex >= 0 {
		fg = lipgloss.Color(strconv.Itoa(colorIndex))
	}
	s := c.Styles.StatusBase.Foreground(fg)

	if width <= 0 {
		return s.Render(left + " " + right)
	}

	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	lw := lipgloss.Width(left)
	rw := lipgloss.Width(right)

	// Ensure at least one space between the two sides.
	gap := max(width-lw-rw, 1)
	filler := strings.Repeat(" ", gap)
	return s.Render(left + filler + right)
}

// CommonKeyMap provides a small set of common key bindings used across views.
type CommonKeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Quit         key.Binding
	ToggleDetail key.Binding
}

func DefaultKeys() CommonKeyMap {
	return CommonKeyMap{
		Up:           key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:         key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Quit:         key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "quit")),
		ToggleDetail: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "toggle details")),
	}
}
