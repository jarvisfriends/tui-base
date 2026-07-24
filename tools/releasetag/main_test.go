package main

import (
	"strings"
	"testing"
)

func TestAnalyzeClassifiesBump(t *testing.T) {
	cases := []struct {
		name      string
		report    string
		wantBump  string
		wantNext  string
		reasonHas string
	}{
		{
			name:      "patch",
			report:    "Inferred base version: v0.1.12\nSuggested version: v0.1.13\n",
			wantBump:  bumpPatch,
			wantNext:  "v0.1.13",
			reasonHas: "backward-compatible patch",
		},
		{
			name:      "minor_added_api",
			report:    "# pkg\n## compatible changes\nNewThing: added\n\nInferred base version: v0.1.12\nSuggested version: v0.2.0\n",
			wantBump:  bumpMinor,
			wantNext:  "v0.2.0",
			reasonHas: "New exported API was added",
		},
		{
			name:      "minor_breaking_v0",
			report:    "# pkg\n## incompatible changes\nOldThing: removed\n\nInferred base version: v0.1.12\nSuggested version: v0.2.0\n",
			wantBump:  bumpMinor,
			wantNext:  "v0.2.0",
			reasonHas: "backward-incompatible",
		},
		{
			name:      "major_breaking_v1",
			report:    "## incompatible changes\nFoo: removed\n\nInferred base version: v1.2.3\nSuggested version: v2.0.0\n",
			wantBump:  bumpMajor,
			wantNext:  "v2.0.0",
			reasonHas: "major version",
		},
		{
			name:     "none",
			report:   "Inferred base version: v0.1.12\nSuggested version: v0.1.12\n",
			wantBump: bumpNone,
			wantNext: "v0.1.12",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := analyze(c.report)
			if a.bump != c.wantBump {
				t.Errorf("bump = %q, want %q", a.bump, c.wantBump)
			}
			if a.next != c.wantNext {
				t.Errorf("next = %q, want %q", a.next, c.wantNext)
			}
			if c.reasonHas != "" && !strings.Contains(a.reason, c.reasonHas) {
				t.Errorf("reason %q does not contain %q", a.reason, c.reasonHas)
			}
			// The rendered markdown must always carry the marker so PR comments
			// are updated in place.
			if !strings.Contains(a.markdown(), commentMarker) {
				t.Error("markdown missing comment marker")
			}
		})
	}
}

func TestParseVer(t *testing.T) {
	for _, c := range []struct {
		in    string
		ok    bool
		major int
	}{
		{"v1.2.3", true, 1},
		{"0.1.12", true, 0},
		{"v2.0.0-rc1", true, 2},
		{"v1.2", false, 0},
		{"nope", false, 0},
	} {
		v, ok := parseVer(c.in)
		if ok != c.ok {
			t.Errorf("parseVer(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && v.major != c.major {
			t.Errorf("parseVer(%q) major = %d, want %d", c.in, v.major, c.major)
		}
	}
}

func TestClassify(t *testing.T) {
	base := ver{0, 1, 12}
	for _, c := range []struct {
		next ver
		want string
	}{
		{ver{0, 1, 13}, bumpPatch},
		{ver{0, 2, 0}, bumpMinor},
		{ver{1, 0, 0}, bumpMajor},
		{ver{0, 1, 12}, bumpNone},
	} {
		if got := classify(base, c.next); got != c.want {
			t.Errorf("classify(%v,%v) = %q, want %q", base, c.next, got, c.want)
		}
	}
}
