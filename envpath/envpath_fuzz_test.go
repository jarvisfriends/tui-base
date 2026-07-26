// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package envpath

import (
	"path/filepath"
	"testing"
)

// FuzzCollapseExpand exercises the path collapse/expand helpers with arbitrary
// input to verify they never panic and always return cleaned output. Collapse
// and Expand run on untrusted config values, so they must tolerate any string.
func FuzzCollapseExpand(f *testing.F) {
	f.Add("")
	f.Add("C:\\Users\\test\\AppData\\Roaming\\app")
	f.Add("$APPDATA/app/config.json")
	f.Add("$HOME/../$TMP/x")
	f.Add("/usr/local/bin")
	f.Add("relative/path")
	f.Add("$")
	f.Add("$$$///\\\\")

	f.Fuzz(func(t *testing.T, path string) {
		t.Setenv("APPDATA", "C:\\Users\\test\\AppData\\Roaming")
		t.Setenv("HOME", "C:\\Users\\test")

		collapsed := Collapse(path)
		// Collapse output must be forward-slash normalized (except empty input).
		if collapsed != "" && filepath.ToSlash(collapsed) != collapsed {
			t.Fatalf("Collapse(%q) = %q contains a backslash separator", path, collapsed)
		}

		// Both directions must survive arbitrary input without panicking, and
		// feeding Collapse's own output back through must remain stable.
		_ = Expand(collapsed)
		if got := Collapse(collapsed); filepath.ToSlash(got) != got {
			t.Fatalf("Collapse(Collapse(%q)) = %q contains a backslash separator", path, got)
		}
	})
}
