package theme

import (
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"

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
		return pairMetrics{failures: []string{"foreground color has zero alpha and cannot be analyzed"}}
	}
	bg, ok := colorful.MakeColor(pair.bg)
	if !ok {
		return pairMetrics{failures: []string{"background color has zero alpha and cannot be analyzed"}}
	}

	metrics := pairMetrics{
		normalContrast: contrastRatio(fg, bg),
		minCVDistance:  math.MaxFloat64,
		minCVContrast:  math.MaxFloat64,
	}

	if metrics.normalContrast < pair.minContrast {
		metrics.failures = append(metrics.failures,
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
			metrics.failures = append(metrics.failures,
				fmt.Sprintf("%s distance %.3f < %.3f", filter.name, distance, pair.minCVDistance),
			)
		}

		contrast := contrastRatio(sfg, sbg)
		if contrast < metrics.minCVContrast {
			metrics.minCVContrast = contrast
		}
		if contrast < pair.minCVContrast {
			metrics.failures = append(metrics.failures,
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

func computeThemeScores() (darkScores []themeScore, lightScores []themeScore) {
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
			t.Logf("mode=%s rank=%d theme=%s (%s) score=%d/%d ratio=%.2f", s.mode, i+1, s.display, s.id, s.passCount, s.totalCount, s.passRatio)
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
			t.Fatalf("theme %q regressed with accessibility mode: base=%d adjusted=%d", tm.ID, basePasses, adjustedPasses)
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
		if err := os.WriteFile(shortlistPath, b, 0o644); err != nil {
			t.Fatalf("write shortlist json: %v", err)
		}
	}

	b, err := os.ReadFile(shortlistPath)
	if err != nil {
		t.Fatalf("read shortlist json: %v", err)
	}

	var got shortlistDoc
	if unmarshalErr := json.Unmarshal(b, &got); unmarshalErr != nil {
		t.Fatalf("parse shortlist json: %v", unmarshalErr)
	}

	if len(got.Dark) == 0 || len(got.Light) == 0 {
		t.Fatalf("shortlist json must include dark and light entries; got dark=%d light=%d", len(got.Dark), len(got.Light))
	}

	want, err := json.Marshal(generated)
	if err != nil {
		t.Fatalf("marshal expected shortlist: %v", err)
	}
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal actual shortlist: %v", err)
	}
	if string(gotJSON) != string(want) {
		t.Fatalf("style combo shortlist out of date; run with UPDATE_STYLE_SHORTLIST=1 to refresh %s", shortlistPath)
	}
}
