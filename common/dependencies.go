package common

import (
	"cmp"
	"fmt"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"time"
)

type Dependency struct {
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
	Replace string `json:"replace,omitempty"`
	Sum     string `json:"sum,omitempty"`
}

type ExpandedInfo struct {
	App struct {
		Path    string `json:"path,omitempty"`
		Version string `json:"version,omitempty"`
	}

	GoVersion string `json:"goVersion"`

	Runtime struct {
		GOOS   string `json:"goos"`
		GOARCH string `json:"goarch"`
		CPUs   int    `json:"cpus"`
	}

	VCS struct {
		Revision string     `json:"revision,omitempty"`
		Time     *time.Time `json:"time,omitempty"`
		Modified *bool      `json:"modified,omitempty"`
	}

	Settings map[string]string `json:"settings,omitempty"`

	Dependencies []Dependency `json:"dependencies"`
}

func Dependencies() []Dependency {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}

	out := make([]Dependency, 0, len(info.Deps))

	for _, dep := range info.Deps {
		d := Dependency{
			Path:    dep.Path,
			Version: dep.Version,
			Sum:     dep.Sum,
		}

		if dep.Replace != nil {
			d.Replace = fmt.Sprintf(
				"%s@%s",
				dep.Replace.Path,
				dep.Replace.Version,
			)
		}

		out = append(out, d)
	}

	slices.SortFunc(out, func(a, b Dependency) int {
		return cmp.Compare(a.Path, b.Path)
	})

	return out
}

func ExpandedBuildInfo() *ExpandedInfo {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}

	var out ExpandedInfo

	out.App.Path = info.Main.Path
	out.App.Version = normalizeVersion(info.Main.Version)
	out.GoVersion = info.GoVersion

	out.Runtime.GOOS = runtime.GOOS
	out.Runtime.GOARCH = runtime.GOARCH
	out.Runtime.CPUs = runtime.NumCPU()

	out.Settings = map[string]string{}

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			out.VCS.Revision = setting.Value

		case "vcs.time":
			if t, err := time.Parse(
				time.RFC3339,
				setting.Value,
			); err == nil {
				out.VCS.Time = &t
			}

		case "vcs.modified":
			b := setting.Value == "true"
			out.VCS.Modified = &b

		default:
			out.Settings[setting.Key] = setting.Value
		}
	}

	out.Dependencies = Dependencies()

	return &out
}

func normalizeVersion(v string) string {
	switch strings.TrimSpace(v) {
	case "", "(devel)":
		return "development"
	default:
		return v
	}
}
