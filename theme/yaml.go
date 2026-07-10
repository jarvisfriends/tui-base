package theme

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tint "github.com/lrstanley/bubbletint/v2"
	"github.com/lucasb-eyer/go-colorful"
	"gopkg.in/yaml.v3"
)

// yamlTint is the on-disk schema for a user-authored theme (T-4, Q-15: the
// full 16-slot terminal tint). Colors are hex strings ("#rgb" or "#rrggbb").
// id, fg, bg, and all sixteen ANSI slots are required; selection_bg and
// cursor are optional; dark is derived from bg luminosity when omitted.
type yamlTint struct {
	ID          string `yaml:"id"`
	DisplayName string `yaml:"display_name"`
	Dark        *bool  `yaml:"dark"`

	Fg          string `yaml:"fg"`
	Bg          string `yaml:"bg"`
	SelectionBg string `yaml:"selection_bg"`
	Cursor      string `yaml:"cursor"`

	Black  string `yaml:"black"`
	Red    string `yaml:"red"`
	Green  string `yaml:"green"`
	Yellow string `yaml:"yellow"`
	Blue   string `yaml:"blue"`
	Purple string `yaml:"purple"`
	Cyan   string `yaml:"cyan"`
	White  string `yaml:"white"`

	BrightBlack  string `yaml:"bright_black"`
	BrightRed    string `yaml:"bright_red"`
	BrightGreen  string `yaml:"bright_green"`
	BrightYellow string `yaml:"bright_yellow"`
	BrightBlue   string `yaml:"bright_blue"`
	BrightPurple string `yaml:"bright_purple"`
	BrightCyan   string `yaml:"bright_cyan"`
	BrightWhite  string `yaml:"bright_white"`
}

// parseHexColor turns "#rgb" / "#rrggbb" into a tint.Color.
func parseHexColor(s string) (*tint.Color, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("empty color")
	}
	if !strings.HasPrefix(s, "#") {
		return nil, fmt.Errorf("color %q must start with '#'", s)
	}
	hexPart := s[1:]
	if len(hexPart) == 3 {
		hexPart = strings.Repeat(string(hexPart[0]), 2) +
			strings.Repeat(string(hexPart[1]), 2) +
			strings.Repeat(string(hexPart[2]), 2)
	}
	if len(hexPart) != 6 {
		return nil, fmt.Errorf("color %q must be #rgb or #rrggbb", s)
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(hexPart, "%02x%02x%02x", &r, &g, &b); err != nil {
		return nil, fmt.Errorf("color %q: %w", s, err)
	}
	return &tint.Color{R: r, G: g, B: b, A: 0xff}, nil
}

// toTint validates the schema and converts it into a registrable tint.
func (y yamlTint) toTint() (*tint.Tint, error) {
	if strings.TrimSpace(y.ID) == "" {
		return nil, errors.New("theme yaml: 'id' is required")
	}
	out := &tint.Tint{
		ID:          y.ID,
		DisplayName: y.DisplayName,
	}
	if out.DisplayName == "" {
		out.DisplayName = y.ID
	}

	required := []struct {
		name string
		hex  string
		dst  **tint.Color
	}{
		{"fg", y.Fg, &out.Fg},
		{"bg", y.Bg, &out.Bg},
		{"black", y.Black, &out.Black},
		{"red", y.Red, &out.Red},
		{"green", y.Green, &out.Green},
		{"yellow", y.Yellow, &out.Yellow},
		{"blue", y.Blue, &out.Blue},
		{"purple", y.Purple, &out.Purple},
		{"cyan", y.Cyan, &out.Cyan},
		{"white", y.White, &out.White},
		{"bright_black", y.BrightBlack, &out.BrightBlack},
		{"bright_red", y.BrightRed, &out.BrightRed},
		{"bright_green", y.BrightGreen, &out.BrightGreen},
		{"bright_yellow", y.BrightYellow, &out.BrightYellow},
		{"bright_blue", y.BrightBlue, &out.BrightBlue},
		{"bright_purple", y.BrightPurple, &out.BrightPurple},
		{"bright_cyan", y.BrightCyan, &out.BrightCyan},
		{"bright_white", y.BrightWhite, &out.BrightWhite},
	}
	for _, f := range required {
		c, err := parseHexColor(f.hex)
		if err != nil {
			return nil, fmt.Errorf("theme %q: %s: %w", y.ID, f.name, err)
		}
		*f.dst = c
	}

	// Optional slots.
	if y.SelectionBg != "" {
		c, err := parseHexColor(y.SelectionBg)
		if err != nil {
			return nil, fmt.Errorf("theme %q: selection_bg: %w", y.ID, err)
		}
		out.SelectionBg = c
	}
	if y.Cursor != "" {
		c, err := parseHexColor(y.Cursor)
		if err != nil {
			return nil, fmt.Errorf("theme %q: cursor: %w", y.ID, err)
		}
		out.Cursor = c
	}

	// Dark: explicit wins; otherwise derived from the background luminance.
	if y.Dark != nil {
		out.Dark = *y.Dark
	} else {
		bg := colorful.Color{
			R: float64(out.Bg.R) / 255, G: float64(out.Bg.G) / 255, B: float64(out.Bg.B) / 255,
		}
		_, _, l := bg.Hsl()
		out.Dark = l < 0.5
	}
	return out, nil
}

// LoadYAMLTints parses every *.yaml/*.yml file in dir as a user theme (T-4).
// A missing directory is not an error (no custom themes). Files that fail to
// parse are reported in errs; valid ones still load, so one bad file never
// hides the rest.
func LoadYAMLTints(dir string) (tints []*tint.Tint, errs []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []error{err}
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path) //nolint:gosec // path is inside the app's own config dir
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		var y yamlTint
		if err = yaml.Unmarshal(data, &y); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		t, err := y.toTint()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		tints = append(tints, t)
	}
	return tints, errs
}

// RegisterYAMLTints loads dir's themes into the global bubbletint registry so
// they appear in the settings Theme selector next to the built-ins. It returns
// how many registered plus any per-file errors. Call it after the registry is
// initialized (the settings page does this on construction with
// <config-dir>/themes).
func RegisterYAMLTints(dir string) (int, []error) {
	tints, errs := LoadYAMLTints(dir)
	for _, t := range tints {
		tint.Register(t)
	}
	return len(tints), errs
}
