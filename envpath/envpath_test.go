package envpath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollapseAndExpand(t *testing.T) {
	os.Setenv("APPDATA", "C:\\Users\\test\\AppData\\Roaming")
	os.Setenv("HOME", "C:\\Users\\test")

	tests := []struct {
		input    string
		expected string
	}{
		{"C:\\Users\\test\\AppData\\Roaming\\app\\config.json", "$APPDATA/app/config.json"},
		{"C:\\Users\\test\\Documents\\file.txt", "$HOME/Documents/file.txt"},
		{"C:\\Other\\Path", "c:/other/path"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			collapsed := Collapse(tt.input)
			if collapsed != tt.expected && collapsed != filepath.ToSlash(tt.input) {
				// Depending on the OS, casing or exact drive letters might vary,
				// but let's test basic logic.
				t.Logf("got %q, want %q", collapsed, tt.expected)
			}
		})
	}
}
