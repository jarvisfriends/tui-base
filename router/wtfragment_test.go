package router

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestFragmentJSON(t *testing.T) {
	data, err := fragmentJSON(WindowsTerminalProfile{
		AppName:           DefaultAppName,
		Commandline:       `"C:\apps\tui-base.exe"`,
		IconPath:          `C:\apps\icon.png`,
		StartingDirectory: "%USERPROFILE%",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Round-trip so the assertions do not depend on formatting.
	var got wtFragment
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("emitted invalid JSON: %v\n%s", err, data)
	}
	if len(got.Profiles) != 1 {
		t.Fatalf("want 1 profile, got %d", len(got.Profiles))
	}
	p := got.Profiles[0]
	if p.Name != DefaultAppName || p.Commandline != `"C:\apps\tui-base.exe"` ||
		p.Icon != `C:\apps\icon.png` || p.StartingDirectory != "%USERPROFILE%" {
		t.Fatalf("profile fields not preserved: %+v", p)
	}
}

func TestFragmentJSONOmitsEmptyOptionals(t *testing.T) {
	data, err := fragmentJSON(WindowsTerminalProfile{AppName: DefaultAppName, Commandline: "run"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "icon") || strings.Contains(s, "startingDirectory") {
		t.Fatalf("empty optionals should be omitted, got:\n%s", s)
	}
}

func TestFragmentFileName(t *testing.T) {
	cases := map[string]string{
		"TUI Base":     "tui-base.json",
		"My Cool App!": "my-cool-app.json",
		"  ":           "app.json",
		"aSettings":    "asettings.json",
	}
	for in, want := range cases {
		if got := fragmentFileName(in); got != want {
			t.Errorf("fragmentFileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQuoteCommand(t *testing.T) {
	if got := quoteCommand(`C:\apps\tui-base.exe`); got != `C:\apps\tui-base.exe` {
		t.Errorf("no-space path should not be quoted, got %q", got)
	}
	if got := quoteCommand(`C:\Program Files\app.exe`); got != `"C:\Program Files\app.exe"` {
		t.Errorf("spaced path should be quoted, got %q", got)
	}
}

func TestWindowsTerminalFragmentDir(t *testing.T) {
	t.Setenv("LOCALAPPDATA", filepath.Join("X:", "Local"))
	dir, err := windowsTerminalFragmentDir("TUI Base")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("X:", "Local", "Microsoft", "Windows Terminal", "Fragments", "TUI Base")
	if dir != want {
		t.Fatalf("dir = %q, want %q", dir, want)
	}

	t.Setenv("LOCALAPPDATA", "")
	if _, err := windowsTerminalFragmentDir("TUI Base"); err == nil {
		t.Fatal("expected error when LOCALAPPDATA is unset")
	}
}
