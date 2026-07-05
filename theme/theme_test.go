package theme

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"
	"github.com/lucasb-eyer/go-colorful"
)

type colorVisionFilter struct {
	name   string
	matrix [3][3]float64
}

type semanticPair struct {
	name          string
	fg            color.Color
	bg            color.Color
	minContrast   float64
	minCVDistance float64
	minCVContrast float64
}

type pairMetrics struct {
	normalContrast float64
	minCVDistance  float64
	minCVContrast  float64
	failures       []string
}

var colorVisionFilters = []colorVisionFilter{
	{
		name: "protanopia",
		matrix: [3][3]float64{
			{0.56667, 0.43333, 0.00000},
			{0.55833, 0.44167, 0.00000},
			{0.00000, 0.24167, 0.75833},
		},
	},
	{
		name: "deuteranopia",
		matrix: [3][3]float64{
			{0.62500, 0.37500, 0.00000},
			{0.70000, 0.30000, 0.00000},
			{0.00000, 0.30000, 0.70000},
		},
	},
	{
		name: "tritanopia",
		matrix: [3][3]float64{
			{0.95000, 0.05000, 0.00000},
			{0.00000, 0.43333, 0.56667},
			{0.00000, 0.47500, 0.52500},
		},
	},
}

func evaluatePair(pair semanticPair) pairMetrics {
	fg, ok := colorful.MakeColor(pair.fg)
	if !ok {
		return pairMetrics{
			failures: []string{"foreground color has zero alpha and cannot be analyzed"},
		}
	}
	bg, ok := colorful.MakeColor(pair.bg)
	if !ok {
		return pairMetrics{
			failures: []string{"background color has zero alpha and cannot be analyzed"},
		}
	}

	metrics := pairMetrics{
		normalContrast: contrastRatio(fg, bg),
		minCVDistance:  math.MaxFloat64,
		minCVContrast:  math.MaxFloat64,
	}

	if metrics.normalContrast < pair.minContrast {
		metrics.failures = append(
			metrics.failures,
			fmt.Sprintf("contrast %.2f < %.2f", metrics.normalContrast, pair.minContrast),
		)
	}

	for _, filter := range colorVisionFilters {
		sfg := applyColorVisionFilter(fg, filter)
		sbg := applyColorVisionFilter(bg, filter)

		distance := sfg.DistanceCIEDE2000(sbg)
		if distance < metrics.minCVDistance {
			metrics.minCVDistance = distance
		}
		if distance < pair.minCVDistance {
			metrics.failures = append(
				metrics.failures,
				fmt.Sprintf("%s distance %.3f < %.3f", filter.name, distance, pair.minCVDistance),
			)
		}

		contrast := contrastRatio(sfg, sbg)
		if contrast < metrics.minCVContrast {
			metrics.minCVContrast = contrast
		}
		if contrast < pair.minCVContrast {
			metrics.failures = append(
				metrics.failures,
				fmt.Sprintf("%s contrast %.2f < %.2f", filter.name, contrast, pair.minCVContrast),
			)
		}
	}

	if metrics.minCVDistance == math.MaxFloat64 {
		metrics.minCVDistance = 0
	}
	if metrics.minCVContrast == math.MaxFloat64 {
		metrics.minCVContrast = 0
	}

	return metrics
}

func applyColorVisionFilter(c colorful.Color, filter colorVisionFilter) colorful.Color {
	r, g, b := c.R, c.G, c.B
	return colorful.Color{
		R: filter.matrix[0][0]*r + filter.matrix[0][1]*g + filter.matrix[0][2]*b,
		G: filter.matrix[1][0]*r + filter.matrix[1][1]*g + filter.matrix[1][2]*b,
		B: filter.matrix[2][0]*r + filter.matrix[2][1]*g + filter.matrix[2][2]*b,
	}.Clamped()
}

func contrastRatio(fg, bg colorful.Color) float64 {
	lf := relativeLuminance(fg)
	lb := relativeLuminance(bg)
	if lf < lb {
		lf, lb = lb, lf
	}
	return (lf + 0.05) / (lb + 0.05)
}

func relativeLuminance(c colorful.Color) float64 {
	linear := func(v float64) float64 {
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}

	r := linear(c.R)
	g := linear(c.G)
	b := linear(c.B)

	return 0.2126*r + 0.7152*g + 0.0722*b
}

func almostEqualFloat64(a, b, epsilon float64) bool {
	return math.Abs(a-b) <= epsilon
}

type themeScore struct {
	id         string
	display    string
	mode       string
	passRatio  float64
	passCount  int
	totalCount int
}

func computeThemeScores() (darkScores, lightScores []themeScore) {
	tint.NewDefaultRegistry()
	tints := tint.DefaultTints()
	if len(tints) == 0 {
		return nil, nil
	}

	for _, tm := range tints {
		app := FromTint(tm)
		combos := StyleCombosFromAppStyle(app)
		if len(combos) == 0 {
			continue
		}

		passCount := 0
		for _, combo := range combos {
			metrics := evaluatePair(semanticPair{
				name:          combo.Name,
				fg:            combo.Fg,
				bg:            combo.Bg,
				minContrast:   3.0,
				minCVDistance: 0.05,
				minCVContrast: 2.5,
			})
			if len(metrics.failures) == 0 {
				passCount++
			}
		}

		score := themeScore{
			id:         tm.ID,
			display:    tm.DisplayName,
			mode:       ThemeModeLight,
			passRatio:  float64(passCount) / float64(len(combos)),
			passCount:  passCount,
			totalCount: len(combos),
		}
		if tm.Dark {
			score.mode = ThemeModeDark
			darkScores = append(darkScores, score)
		} else {
			lightScores = append(lightScores, score)
		}
	}

	stableSort := func(scores []themeScore) {
		sort.Slice(scores, func(i, j int) bool {
			if !almostEqualFloat64(scores[i].passRatio, scores[j].passRatio, 1e-9) {
				return scores[i].passRatio > scores[j].passRatio
			}
			if scores[i].passCount != scores[j].passCount {
				return scores[i].passCount > scores[j].passCount
			}
			return scores[i].id < scores[j].id
		})
	}
	stableSort(darkScores)
	stableSort(lightScores)
	return darkScores, lightScores
}

func topScores(scores []themeScore, n int) []themeScore {
	if n <= 0 || len(scores) == 0 {
		return nil
	}
	if n > len(scores) {
		n = len(scores)
	}
	out := make([]themeScore, n)
	copy(out, scores[:n])
	return out
}

func TestDefaultStyleComboAccessibilityReport(t *testing.T) {
	darkScores, lightScores := computeThemeScores()
	if len(darkScores) == 0 && len(lightScores) == 0 {
		t.Fatal("no default tints available")
	}

	reportTop := func(scores []themeScore, mode string) {
		if len(scores) == 0 {
			t.Logf("mode=%s: no themes discovered", mode)
			return
		}
		limit := min(8, len(scores))
		for i := range limit {
			s := scores[i]
			t.Logf(
				"mode=%s rank=%d theme=%s (%s) score=%d/%d ratio=%.2f",
				s.mode,
				i+1,
				s.display,
				s.id,
				s.passCount,
				s.totalCount,
				s.passRatio,
			)
		}
	}

	reportTop(darkScores, ThemeModeDark)
	reportTop(lightScores, ThemeModeLight)

	if len(darkScores) == 0 {
		t.Fatal("expected at least one dark theme in default tint registry")
	}
	if len(lightScores) == 0 {
		t.Fatal("expected at least one light theme in default tint registry")
	}
}

func TestAccessibilityOptionImprovesOrMatchesStyleCombos(t *testing.T) {
	tint.NewDefaultRegistry()
	tints := tint.DefaultTints()
	if len(tints) == 0 {
		t.Fatal("no default tints available")
	}

	for _, tm := range tints {
		base := FromTintWithOptions(tm, false)
		adjusted := FromTintWithOptions(tm, true)

		baseCombos := StyleCombosFromAppStyle(base)
		adjustedCombos := StyleCombosFromAppStyle(adjusted)
		if len(baseCombos) == 0 || len(adjustedCombos) == 0 {
			continue
		}

		basePasses := 0
		for _, combo := range baseCombos {
			metrics := evaluatePair(semanticPair{
				name:          combo.Name,
				fg:            combo.Fg,
				bg:            combo.Bg,
				minContrast:   3.0,
				minCVDistance: 0.05,
				minCVContrast: 2.5,
			})
			if len(metrics.failures) == 0 {
				basePasses++
			}
		}

		adjustedPasses := 0
		for _, combo := range adjustedCombos {
			metrics := evaluatePair(semanticPair{
				name:          combo.Name,
				fg:            combo.Fg,
				bg:            combo.Bg,
				minContrast:   3.0,
				minCVDistance: 0.05,
				minCVContrast: 2.5,
			})
			if len(metrics.failures) == 0 {
				adjustedPasses++
			}
		}

		if adjustedPasses < basePasses {
			t.Fatalf(
				"theme %q regressed with accessibility mode: base=%d adjusted=%d",
				tm.ID,
				basePasses,
				adjustedPasses,
			)
		}
	}
}

type shortlistTheme struct {
	ID         string  `json:"id"`
	Display    string  `json:"display"`
	PassRatio  float64 `json:"pass_ratio"`
	PassCount  int     `json:"pass_count"`
	TotalCount int     `json:"total_count"`
}

type shortlistDoc struct {
	Dark  []shortlistTheme `json:"dark"`
	Light []shortlistTheme `json:"light"`
}

func toShortlistThemes(scores []themeScore) []shortlistTheme {
	out := make([]shortlistTheme, len(scores))
	for i, s := range scores {
		out[i] = shortlistTheme{
			ID:         s.id,
			Display:    s.display,
			PassRatio:  s.passRatio,
			PassCount:  s.passCount,
			TotalCount: s.totalCount,
		}
	}
	return out
}

func TestStyleComboShortlistJSON(t *testing.T) {
	darkScores, lightScores := computeThemeScores()
	if len(darkScores) == 0 || len(lightScores) == 0 {
		t.Fatal("unable to compute theme scores for shortlist")
	}

	generated := shortlistDoc{
		Dark:  toShortlistThemes(topScores(darkScores, 8)),
		Light: toShortlistThemes(topScores(lightScores, 8)),
	}

	shortlistPath := filepath.Join(".", "style_combo_shortlist.json")
	if os.Getenv("UPDATE_STYLE_SHORTLIST") == "1" {
		b, err := json.MarshalIndent(generated, "", "  ")
		if err != nil {
			t.Fatalf("marshal generated shortlist: %v", err)
		}
		b = append(b, '\n')
		if err := os.WriteFile(shortlistPath, b, 0o600); err != nil {
			t.Fatalf("write shortlist json: %v", err)
		}
	}

	b, err := os.ReadFile(filepath.Clean(shortlistPath))
	if err != nil {
		t.Fatalf("read shortlist json: %v", err)
	}

	var got shortlistDoc
	if unmarshalErr := json.Unmarshal(b, &got); unmarshalErr != nil {
		t.Fatalf("parse shortlist json: %v", unmarshalErr)
	}

	if len(got.Dark) == 0 || len(got.Light) == 0 {
		t.Fatalf(
			"shortlist json must include dark and light entries; got dark=%d light=%d",
			len(got.Dark),
			len(got.Light),
		)
	}

	want, err := json.Marshal(generated)
	if err != nil {
		t.Fatalf("marshal expected shortlist: %v", err)
	}
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal actual shortlist: %v", err)
	}
	if !bytes.Equal(gotJSON, want) {
		t.Fatalf(
			"style combo shortlist out of date; run with UPDATE_STYLE_SHORTLIST=1 to refresh %s",
			shortlistPath,
		)
	}
}

func TestThemeGlobalStateAndAccessors(t *testing.T) {
	// 1. NormalizeMode
	if NormalizeMode("light") != ThemeModeLight {
		t.Errorf("NormalizeMode(light) = %q; want %q", NormalizeMode("light"), ThemeModeLight)
	}
	if NormalizeMode("LIGHT  ") != ThemeModeLight {
		t.Errorf("expected LIGHT to normalize to light")
	}
	if NormalizeMode("dark") != ThemeModeDark {
		t.Errorf("NormalizeMode(dark) = %q; want %q", NormalizeMode("dark"), ThemeModeDark)
	}
	if NormalizeMode("invalid") != ThemeModeDark {
		t.Errorf("expected invalid mode to normalize to dark")
	}

	// 2. SetThemePreferences & ThemePreferencesSnapshot
	SetThemePreferences("light", true, PresetDracula)
	snap := ThemePreferencesSnapshot()
	if snap.Mode != ThemeModeLight || !snap.Accessibility || snap.Style != PresetDracula {
		t.Errorf("unexpected preferences snapshot: %+v", snap)
	}

	// 3. ResolveTintIDForMode
	tints := tint.Tints()
	if len(tints) > 0 {
		verifyTintColors(t, tints)
	}

	// 4. SetCurrentTint
	err := SetCurrentTint("dracula")
	if err != nil {
		t.Errorf("SetCurrentTint(dracula) failed: %v", err)
	}
	err = SetCurrentTint("invalid-tint-id")
	if err == nil {
		t.Error("expected SetCurrentTint to fail for invalid tint id")
	}
	err = SetCurrentTint("")
	if err != nil {
		t.Errorf("expected no-op for empty tint id, got: %v", err)
	}

	// 5. Active
	activeStyle := Active()
	if activeStyle == nil {
		t.Fatal("expected non-nil active style")
	}

	// col and borderColor
	cVal := col(nil, "240")
	if cVal != lipgloss.Color("240") {
		t.Errorf("col(nil, 240) = %v; want 240", cVal)
	}

	testTintDark := &tint.Tint{
		ID:   "test-dark",
		Dark: true,
	}
	bcDark := borderColor(testTintDark)
	if bcDark == nil {
		t.Error("expected non-nil dark border color")
	}

	testTintLight := &tint.Tint{
		ID:   "test-light",
		Dark: false,
	}
	bcLight := borderColor(testTintLight)
	if bcLight == nil {
		t.Error("expected non-nil light border color")
	}

	// 6. render.go helpers
	// ColorHex
	h := ColorHex(lipgloss.Color("#ff0000"))
	if h != "#ff0000" {
		t.Errorf("ColorHex(#ff0000) = %q; want #ff0000", h)
	}
	hNil := ColorHex(nil)
	if hNil != "#000000" {
		t.Errorf("ColorHex(nil) = %q; want #000000", hNil)
	}

	// ReapplyBg
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	reapplied := ReapplyBg("hello\x1b[0mworld\x1b[m", style.GetForeground())
	if !strings.Contains(reapplied, "\x1b[0m") || !strings.Contains(reapplied, "\x1b[m") {
		t.Errorf("expected escape sequences to be preserved: %q", reapplied)
	}
	// Also test ReapplyBg with bg color returning empty bgCode
	if got := ReapplyBg("hello", nil); got != "hello" {
		t.Errorf("expected original string when bg is nil, got %q", got)
	}

	// firstEscapeFromStyle boundary
	if got := firstEscapeFromStyle("no-escape"); got != "" {
		t.Errorf("expected empty for no-escape, got %q", got)
	}
	if got := firstEscapeFromStyle("\x1b[no-ending"); got != "" {
		t.Errorf("expected empty for no-ending escape, got %q", got)
	}
}

func verifyTintColors(t *testing.T, tints []*tint.Tint) {
	t.Helper()
	firstDark := ""
	firstLight := ""
	for _, tm := range tints {
		if tm.Dark && firstDark == "" {
			firstDark = tm.ID
		} else if !tm.Dark && firstLight == "" {
			firstLight = tm.ID
		}
	}
	if firstDark != "" {
		res := ResolveTintIDForMode(firstDark, "dark")
		if res != firstDark {
			t.Errorf("ResolveTintIDForMode(%s, dark) = %q; want %s", firstDark, res, firstDark)
		}
		res2 := ResolveTintIDForMode("", "dark")
		if res2 != firstDark {
			t.Errorf("ResolveTintIDForMode('', dark) = %q; want %s", res2, firstDark)
		}
		res3 := ResolveTintIDForMode("some-invalid-id", "dark")
		if res3 != firstDark {
			t.Errorf("expected fallback for invalid id to first dark tint, got %s", res3)
		}
	}
	if firstLight != "" {
		res := ResolveTintIDForMode(firstLight, "light")
		if res != firstLight {
			t.Errorf("ResolveTintIDForMode(%s, light) = %q; want %s", firstLight, res, firstLight)
		}
	}
}

func TestThemeStatus(t *testing.T) {
	// Set theme to a known one first
	_ = SetCurrentTint("dracula")

	// BoxStyle, BoxTitleStyle, SubtleStyle
	bs := BoxStyle()
	if bs.GetPaddingLeft() <= 0 {
		t.Errorf("expected BoxStyle padding")
	}
	bts := BoxTitleStyle()
	if bts.GetForeground() == nil {
		t.Errorf("expected BoxTitleStyle foreground")
	}
	ss := SubtleStyle()
	if ss.GetForeground() == nil {
		t.Errorf("expected SubtleStyle foreground")
	}

	// RenderStatusBar
	bar1 := RenderStatusBar(0, "left", "right")
	if !strings.Contains(bar1, "left") || !strings.Contains(bar1, "right") {
		t.Errorf("RenderStatusBar(0) = %q; missing left/right", bar1)
	}

	bar2 := RenderStatusBar(80, "left", "right")
	if lipgloss.Width(bar2) != 80 {
		t.Errorf("expected RenderStatusBar(80) width to be 80; got %d", lipgloss.Width(bar2))
	}

	bar3 := RenderStatusBarStyled(80, "left", "right", 9)
	if !strings.Contains(bar3, "left") {
		t.Errorf("RenderStatusBarStyled = %q; missing left", bar3)
	}

	// DefaultKeys
	km := DefaultKeys()
	if len(km.Up.Keys()) == 0 {
		t.Errorf("expected Up keys")
	}

	// Preset DisplayNames
	if got := PresetBase.DisplayName(); got != "Base" {
		t.Errorf("expected Base, got %s", got)
	}
	if got := PresetCharm.DisplayName(); got != "Charm" {
		t.Errorf("expected Charm, got %s", got)
	}
	if got := PresetDracula.DisplayName(); got != "Dracula" {
		t.Errorf("expected Dracula, got %s", got)
	}
	if got := PresetBase16.DisplayName(); got != "Base16" {
		t.Errorf("expected Base16, got %s", got)
	}
	if got := PresetCatppuccin.DisplayName(); got != "Catppuccin" {
		t.Errorf("expected Catppuccin, got %s", got)
	}
	if got := StylePreset("invalid").DisplayName(); got != "invalid" {
		t.Errorf("expected invalid, got %s", got)
	}

	// HuhThemeFunc
	tf := HuhThemeFunc()
	if tf == nil {
		t.Error("expected non-nil HuhThemeFunc")
	}
	styles := tf(true)
	if styles == nil {
		t.Error("expected non-nil styles from theme function")
	}

	// AccessiblePairsFromTint
	tints2 := tint.Tints()
	if len(tints2) > 0 {
		pairs := AccessiblePairsFromTint(tints2[0])
		if len(pairs) == 0 {
			t.Error("expected non-empty accessible pairs")
		}
	}

	// colorPairsFromSimple with nil Tint
	pairsNil := colorPairsFromTint(nil, false)
	if len(pairsNil) == 0 {
		t.Error("expected non-empty simple pairs")
	}
}

func TestThemeContrastSafeguards(t *testing.T) {
	tint.NewDefaultRegistry()
	tints := tint.DefaultTints()
	if len(tints) == 0 {
		t.Fatal("no default tints available")
	}

	for _, tm := range tints {
		app := FromTint(tm)

		// 1. Primary Fg vs Bg must not be identical
		if app.Fg == app.Bg {
			t.Errorf("theme %s: primary Fg and Bg are identical", tm.ID)
		}

		// 2. SelectionBg and Bg must not be identical
		if app.SelectionBg == app.Bg {
			t.Errorf("theme %s: SelectionBg and Bg are identical", tm.ID)
		}

		// 3. SelectionBg and Bg luminance must be sufficiently far apart
		bgL := colorLuminance(app.Bg)
		selL := colorLuminance(app.SelectionBg)
		diff := math.Abs(bgL - selL)
		// We expect the adjustment code in theme.go to shift selection bg away by at least 15% (scaled out of 255)
		// which means a difference of at least ~15 units.
		if diff < 10.0 {
			t.Errorf(
				"theme %s: selection bg %v and background %v are too close (luminance diff %.2f)",
				tm.ID,
				app.SelectionBg,
				app.Bg,
				diff,
			)
		}

		// 4. Accessibility mode adjustments verification
		adjusted := FromTintWithOptions(tm, true)
		if adjusted.Fg == adjusted.Bg {
			t.Errorf("theme %s: adjusted Fg and Bg are identical", tm.ID)
		}
	}
}
