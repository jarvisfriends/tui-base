package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validTheme = `
id: test-ocean
display_name: Test Ocean
fg: "#e6e6e6"
bg: "#0a1930"
selection_bg: "#254a7d"
cursor: "#ffcc00"
black: "#000000"
red: "#ff5555"
green: "#50fa7b"
yellow: "#f1fa8c"
blue: "#6272a4"
purple: "#bd93f9"
cyan: "#8be9fd"
white: "#f8f8f2"
bright_black: "#44475a"
bright_red: "#ff6e6e"
bright_green: "#69ff94"
bright_yellow: "#ffffa5"
bright_blue: "#d6acff"
bright_purple: "#ff92df"
bright_cyan: "#a4ffff"
bright_white: "#ffffff"
`

func writeTheme(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadYAMLTintsParsesFullSchema(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTheme(t, dir, "ocean.yaml", validTheme)

	tints, errs := LoadYAMLTints(dir)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(tints) != 1 {
		t.Fatalf("loaded %d tints; want 1", len(tints))
	}
	tt := tints[0]
	if tt.ID != "test-ocean" || tt.DisplayName != "Test Ocean" {
		t.Fatalf("identity fields wrong: %q / %q", tt.ID, tt.DisplayName)
	}
	if tt.Bg.R != 0x0a || tt.Bg.G != 0x19 || tt.Bg.B != 0x30 {
		t.Fatalf("bg parsed wrong: %+v", tt.Bg)
	}
	if tt.BrightPurple.R != 0xff || tt.BrightPurple.G != 0x92 || tt.BrightPurple.B != 0xdf {
		t.Fatalf("bright_purple parsed wrong: %+v", tt.BrightPurple)
	}
	if tt.SelectionBg == nil || tt.Cursor == nil {
		t.Fatal("optional slots missing despite being present in the file")
	}
	if !tt.Dark {
		t.Fatal("dark not derived from the near-black background")
	}
}

func TestLoadYAMLTintsShortHexAndExplicitDark(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	body := validTheme + "\ndark: false\n"
	body = strings.Replace(body, `bg: "#0a1930"`, `bg: "#012"`, 1)
	writeTheme(t, dir, "short.yml", body)

	tints, errs := LoadYAMLTints(dir)
	if len(errs) != 0 || len(tints) != 1 {
		t.Fatalf("load: tints=%d errs=%v", len(tints), errs)
	}
	if tints[0].Bg.R != 0x00 || tints[0].Bg.G != 0x11 || tints[0].Bg.B != 0x22 {
		t.Fatalf("#rgb expansion wrong: %+v", tints[0].Bg)
	}
	if tints[0].Dark {
		t.Fatal("explicit dark: false must win over luminance derivation")
	}
}

func TestLoadYAMLTintsBadFileDoesNotHideGoodOnes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTheme(t, dir, "good.yaml", validTheme)
	writeTheme(t, dir, "bad.yaml", "id: broken\nfg: notacolor\n")
	writeTheme(t, dir, "ignored.txt", "not yaml")

	tints, errs := LoadYAMLTints(dir)
	if len(tints) != 1 {
		t.Fatalf("good theme lost: %d tints", len(tints))
	}
	if len(errs) != 1 {
		t.Fatalf("want exactly one error for bad.yaml, got %v", errs)
	}
}

func TestLoadYAMLTintsMissingDirIsNotAnError(t *testing.T) {
	t.Parallel()

	tints, errs := LoadYAMLTints(filepath.Join(t.TempDir(), "nope"))
	if len(tints) != 0 || len(errs) != 0 {
		t.Fatalf("missing dir must be a silent no-op, got tints=%d errs=%v", len(tints), errs)
	}
}
