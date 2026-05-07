package debug

// accessibility.go — filterable color-accessibility theme browser panel.
// Implements tea.Model. Embedded in debug.Model; toggle with the 'a' key.
//
// Key bindings (when panel is visible):
//   up/down  navigate theme list
//   1/2/3    toggle Protanopia / Deuteranopia / Tritanopia filter
//   d / l    toggle Dark / Light theme inclusion
//   enter    apply highlighted theme to all pages (settings.ThemeMsg)
//
// Filter semantics: with 0 CVD filters, only fully-accessible themes are shown.
// Each additional CVD filter adds themes that fail for that deficiency type,
// so more checks → more themes visible.

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/jarvisfriends/tui-base/pages/settings"
	"github.com/jarvisfriends/tui-base/theme"

	tea "charm.land/bubbletea/v2"
	tint "github.com/lrstanley/bubbletint/v2"
	"github.com/lucasb-eyer/go-colorful"
)

// ── CVD simulation matrices ──────────────────────────────────────────────────

type acvdMat [3][3]float64

var (
	acvdMats = [3]acvdMat{
		{{0.56667, 0.43333, 0}, {0.55833, 0.44167, 0}, {0, 0.24167, 0.75833}}, // protanopia
		{{0.625, 0.375, 0}, {0.7, 0.3, 0}, {0, 0.3, 0.7}},                     // deuteranopia
		{{0.95, 0.05, 0}, {0, 0.43333, 0.56667}, {0, 0.475, 0.525}},           // tritanopia
	}
	acvdNames = [3]string{"Protanopia (Red)", "Deuteranopia (Green)", "Tritanopia (Blue)"}
)

func acvdApply(c colorful.Color, m acvdMat) colorful.Color {
	return colorful.Color{
		R: m[0][0]*c.R + m[0][1]*c.G + m[0][2]*c.B,
		G: m[1][0]*c.R + m[1][1]*c.G + m[1][2]*c.B,
		B: m[2][0]*c.R + m[2][1]*c.G + m[2][2]*c.B,
	}.Clamped()
}

func acvdLuminance(c colorful.Color) float64 {
	lin := func(v float64) float64 {
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

func acvdContrast(fg, bg colorful.Color) float64 {
	lf, lb := acvdLuminance(fg), acvdLuminance(bg)
	if lf < lb {
		lf, lb = lb, lf
	}
	return (lf + 0.05) / (lb + 0.05)
}

// suggestFG blends the foreground toward black then white until it reaches
// minContrast against bg. Returns hex string, or "" if none found.“
func suggestFG(fg, bg color.Color, minContrast float64) (sugFg color.Color, sugFgHex string) {
	fgC, ok := colorful.MakeColor(fg)
	if !ok {
		return nil, ""
	}
	bgC, ok := colorful.MakeColor(bg)
	if !ok {
		return nil, ""
	}
	for _, target := range []colorful.Color{{R: 0, G: 0, B: 0}, {R: 1, G: 1, B: 1}} {
		for blend := 0.0; blend <= 1.0; blend += 0.02 {
			c := fgC.BlendLab(target, blend).Clamped()
			if acvdContrast(c, bgC) >= minContrast {
				return &c, c.Hex()
			}
		}
	}
	return nil, ""
}

// ── Semantic color pairs ─────────────────────────────────────────────────────

type acPair struct {
	name          string
	fg, bg        func(*theme.AppStyle) color.Color
	minContrast   float64
	minCVContrast float64
}

var acPairs = []acPair{
	{"Fg/Bg", func(c *theme.AppStyle) color.Color { return c.Styles.TextOnBg.GetForeground() }, func(c *theme.AppStyle) color.Color { return c.Styles.TextOnBg.GetBackground() }, 4.5, 3.0},
	{"Muted/Bg", func(c *theme.AppStyle) color.Color { return c.Styles.Subtitle.GetForeground() }, func(c *theme.AppStyle) color.Color { return c.Styles.TextOnBg.GetBackground() }, 3.0, 2.5},
	{"Accent/Bg", func(c *theme.AppStyle) color.Color { return c.Styles.Title.GetForeground() }, func(c *theme.AppStyle) color.Color { return c.Styles.TextOnBg.GetBackground() }, 3.0, 2.5},
	{"Sel/SelBg", func(c *theme.AppStyle) color.Color { return c.Styles.SelectedItem.GetForeground() }, func(c *theme.AppStyle) color.Color { return c.Styles.SelectedItem.GetBackground() }, 4.5, 3.0},
	{"Status", func(c *theme.AppStyle) color.Color { return c.Styles.StatusBase.GetForeground() }, func(c *theme.AppStyle) color.Color { return c.Styles.StatusBase.GetBackground() }, 4.5, 3.0},
	{"Success/Bg", func(c *theme.AppStyle) color.Color { return c.Styles.Success.GetForeground() }, func(c *theme.AppStyle) color.Color { return c.Styles.TextOnBg.GetBackground() }, 3.0, 2.5},
	{"Error/Bg", func(c *theme.AppStyle) color.Color { return c.Styles.Error.GetForeground() }, func(c *theme.AppStyle) color.Color { return c.Styles.TextOnBg.GetBackground() }, 3.0, 2.5},
	{"Warning/Bg", func(c *theme.AppStyle) color.Color { return c.Styles.Warning.GetForeground() }, func(c *theme.AppStyle) color.Color { return c.Styles.TextOnBg.GetBackground() }, 3.0, 2.5},
}

// ── Entry / issue types ──────────────────────────────────────────────────────

// acIssue is one failing pair for a tint.
type acIssue struct {
	pair           string
	reason         string
	suggestedFg    color.Color // suggested foreground color, nil when no fix found
	suggestedFgHex string      // hex string of the suggested foreground color
	Bg             color.Color // background color for which the fg failed; used when showing the suggestion
	oldFg          color.Color // the original foreground color; used when showing the suggestion
}

// acEntry holds pre-computed accessibility results for one tint.
type acEntry struct {
	t                *tint.Tint
	normFail         bool
	cvdFail          [3]bool
	issues           []acIssue         // normal-vision failures (with suggestions)
	colorPairsOrig   []theme.ColorPair // original color pairs from the theme
	colorPairsAccess []theme.ColorPair // accessible color pairs (with adjustments applied)
}

// ── AccessibilityPanel model ─────────────────────────────────────────────────

// AccessibilityPanel is a tea.Model embedded in the debug inspector page.
type AccessibilityPanel struct {
	colors    *theme.AppStyle
	width     int
	height    int
	visible   bool
	showDark  bool
	showLight bool
	cvd       [3]bool // per-type inclusion filter
	all       []acEntry
	computed  bool
	view      []int // indices into all; rebuilt by refilter()
	cursor    int
	offset    int
}

// NewAccessibilityPanel returns a panel with dark+light themes shown.
func NewAccessibilityPanel() *AccessibilityPanel {
	return &AccessibilityPanel{showDark: true, showLight: true}
}

func (p *AccessibilityPanel) SetColors(c *theme.AppStyle) { p.colors = c }
func (p *AccessibilityPanel) SetSize(w, h int)            { p.width = w; p.height = h }
func (p *AccessibilityPanel) IsVisible() bool             { return p.visible }

// Toggle shows/hides the panel, computing results on first open.
func (p *AccessibilityPanel) Toggle() {
	p.visible = !p.visible
	if p.visible && !p.computed {
		p.computeAll()
	}
	if p.visible {
		p.refilter()
	}
}

func (p *AccessibilityPanel) Init() tea.Cmd { return nil }

func (p *AccessibilityPanel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !p.visible {
		return p, nil
	}
	press, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	refilter := false
	switch press.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
	case "down", "j":
		if p.cursor < len(p.view)-1 {
			p.cursor++
		}
	case "1":
		p.cvd[0] = !p.cvd[0]
		refilter = true
	case "2":
		p.cvd[1] = !p.cvd[1]
		refilter = true
	case "3":
		p.cvd[2] = !p.cvd[2]
		refilter = true
	case "d":
		p.showDark = !p.showDark
		refilter = true
	case "l":
		p.showLight = !p.showLight
		refilter = true
	case "enter":
		if p.cursor >= 0 && p.cursor < len(p.view) {
			id := p.all[p.view[p.cursor]].t.ID
			return p, func() tea.Msg { return settings.ThemeMsg{ID: id} }
		}
	}
	if refilter {
		p.refilter()
		p.cursor = 0
		p.offset = 0
	}
	return p, nil
}

// Render returns the panel as a string for embedding in the debug page View.
func (p *AccessibilityPanel) View() tea.View {
	if !p.visible {
		return tea.NewView("")
	}
	c := theme.Active()
	if p.colors != nil {
		c = p.colors
	}
	accent := c.Styles.Title
	muted := c.Styles.Subtitle
	good := c.Styles.Success
	bad := c.Styles.Error
	selStyle := c.Styles.SelectedItem
	divider := strings.Repeat("─", min(p.width, 80))

	checkbox := func(on bool, key, label string) string {
		if on {
			return muted.Render(key+":") + good.Render("[✓]") + " " + label
		}
		return muted.Render(key+":") + "[ ] " + label
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s\n",
		accent.Render("COLOR ACCESSIBILITY BROWSER"),
		muted.Render("[a] close  [↑↓] navigate  [enter] apply theme"),
	)
	fmt.Fprintf(&b, "%s   %s    %s   %s   %s\n",
		checkbox(p.showDark, "d", "Dark"),
		checkbox(p.showLight, "l", "Light"),
		checkbox(p.cvd[0], "1", acvdNames[0]),
		checkbox(p.cvd[1], "2", acvdNames[1]),
		checkbox(p.cvd[2], "3", acvdNames[2]),
	)
	fmt.Fprintf(&b, "%s\n", muted.Render(fmt.Sprintf("%d themes shown", len(p.view))))
	fmt.Fprintf(&b, "%s\n", divider)

	// Scrollable list — keep cursor visible.
	// rows consumed above
	listH := max(p.height-4, 3)
	if p.cursor < p.offset {
		p.offset = p.cursor
	}
	if p.cursor >= p.offset+listH {
		p.offset = p.cursor - listH + 1
	}
	end := min(p.offset+listH, len(p.view))
	for vi := p.offset; vi < end; vi++ {
		e := p.all[p.view[vi]]

		// Pass/fail badges: N=normal vision, P/D/T = CVD types
		normBadge := good.Render("N✓")
		if e.normFail {
			normBadge = bad.Render("N✗")
		}
		cvdBadges := make([]string, 3)
		for j, letter := range [3]string{"P", "D", "T"} {
			if e.cvdFail[j] {
				cvdBadges[j] = bad.Render(letter + "✗")
			} else {
				cvdBadges[j] = good.Render(letter + "✓")
			}
		}
		badges := normBadge + " " + strings.Join(cvdBadges[:], " ")

		// Theme name rendered with its own fg/bg for a live swatch effect.
		darkIcon := "☾"
		if !e.t.Dark {
			darkIcon = "☀"
		}
		nameStyle := c.Styles.SwatchDot
		if e.t.Fg != nil {
			nameStyle = nameStyle.Foreground(e.t.Fg)
		}
		if e.t.Bg != nil {
			nameStyle = nameStyle.Background(e.t.Bg)
		}
		name := darkIcon + " " + nameStyle.Render(fmt.Sprintf("%-28s", e.t.DisplayName))
		row := name + "  " + badges

		if vi == p.cursor {
			// Highlight selected row, then show issue details and color palette beneath it.
			fmt.Fprintf(&b, "▶ %s\n", selStyle.Render(row))

			// Show accessibility issues
			for _, issue := range e.issues {
				fmt.Fprint(&b, muted.Render("  ↳ "+issue.pair+": "), bad.Render(issue.reason))
				if issue.suggestedFg != nil {
					oldStyle := c.Styles.SwatchDot.Background(issue.Bg).Foreground(issue.oldFg)
					sugStyle := c.Styles.SwatchDot.Background(issue.Bg).Foreground(issue.suggestedFg)
					fmt.Fprint(&b, "  → ", " ", good.Render(issue.suggestedFgHex), oldStyle.Render(" (original)"), sugStyle.Render(" (suggested)"))
				}
				b.WriteString("\n")
			}

			// Show color palette if we have color pairs
			if len(e.colorPairsOrig) > 0 {
				b.WriteString(muted.Render("  Orig colors:  "))
				for i, pair := range e.colorPairsOrig {
					if i%8 == 0 {
						b.WriteString(" ") // separator between fg/bg groups
					}
					pairStyle := c.Styles.SwatchDot.Background(pair.Bg).Foreground(pair.Fg)
					b.WriteString(pairStyle.Render("●"))
				}
				b.WriteString("\n")

				b.WriteString(muted.Render("  Accessible:   "))
				for i, pair := range e.colorPairsAccess {
					if i%8 == 0 {
						b.WriteString(" ") // separator between fg/bg groups
					}
					pairStyle := c.Styles.SwatchDot.Background(pair.Bg).Foreground(pair.Fg)
					b.WriteString(pairStyle.Render("●"))
				}
				b.WriteString("\n")
			}
		} else {
			fmt.Fprintf(&b, "  %s\n", row)
		}
	}
	return tea.NewView(b.String())
}

// ── Computation ──────────────────────────────────────────────────────────────

func (p *AccessibilityPanel) computeAll() {
	allTints := tint.DefaultTints()
	p.all = make([]acEntry, 0, len(allTints))
	for _, t := range allTints {
		colors := theme.FromTint(t)
		entry := acEntry{
			t:                t,
			colorPairsOrig:   colors.OrigPairs,
			colorPairsAccess: theme.AccessiblePairsFromTint(t),
		}

		// Normal-vision check.
		for _, pair := range acPairs {
			fgC, ok1 := colorful.MakeColor(pair.fg(colors))
			bgC, ok2 := colorful.MakeColor(pair.bg(colors))
			if !ok1 || !ok2 {
				continue
			}
			ratio := acvdContrast(fgC, bgC)
			if ratio < pair.minContrast {
				entry.normFail = true
				sug, sugHex := suggestFG(pair.fg(colors), pair.bg(colors), pair.minContrast)
				entry.issues = append(entry.issues, acIssue{
					pair:           pair.name,
					reason:         fmt.Sprintf("contrast %.1f < %.1f", ratio, pair.minContrast),
					suggestedFg:    sug,
					suggestedFgHex: sugHex,
					Bg:             bgC,
					oldFg:          fgC,
				})
			}
		}

		// CVD-type checks.
		for j, mat := range acvdMats {
			for _, pair := range acPairs {
				fgC, ok1 := colorful.MakeColor(pair.fg(colors))
				bgC, ok2 := colorful.MakeColor(pair.bg(colors))
				if !ok1 || !ok2 {
					continue
				}
				sfg := acvdApply(fgC, mat)
				sbg := acvdApply(bgC, mat)
				if acvdContrast(sfg, sbg) < pair.minCVContrast {
					entry.cvdFail[j] = true
					break // one failing pair is enough to mark the type
				}
			}
		}

		p.all = append(p.all, entry)
	}
	p.computed = true
}

func (p *AccessibilityPanel) refilter() {
	p.view = p.view[:0]
	anyCVD := p.cvd[0] || p.cvd[1] || p.cvd[2]
	for i, e := range p.all {
		if e.t.Dark && !p.showDark {
			continue
		}
		if !e.t.Dark && !p.showLight {
			continue
		}
		allPass := !e.normFail && !e.cvdFail[0] && !e.cvdFail[1] && !e.cvdFail[2]
		if !anyCVD {
			// Show only fully-accessible themes.
			if allPass {
				p.view = append(p.view, i)
			}
			continue
		}
		// Always show safe themes; additionally show themes that fail for
		// any of the checked CVD types.
		if allPass {
			p.view = append(p.view, i)
			continue
		}
		if e.normFail {
			p.view = append(p.view, i)
			continue
		}
		for j, checked := range p.cvd {
			if checked && e.cvdFail[j] {
				p.view = append(p.view, i)
				break
			}
		}
	}
}
