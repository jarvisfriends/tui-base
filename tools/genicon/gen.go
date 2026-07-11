package main

// Regenerate the tui-base app icon and the embedded Windows resource from the
// master SVG. Run it from this directory:
//
//	go generate .        # from tools/genicon
//	go -C tools/genicon generate .   # from the repo root
//
// It is deliberately scoped to this module rather than the repo-root
// `go generate ./...`, so the always-run CI drift check and goreleaser build
// never depend on the SVG toolchain — the committed assets/icon.ico and
// cmd/tui-base/resource_windows_*.syso are the source of truth. Rerun this only
// when assets/icon.svg changes, and commit the regenerated files.
//
//go:generate go run . -svg ../../assets/icon.svg -ico ../../assets/icon.ico -png ../../cmd/tui-base/tabicon.png -syso ../../cmd/tui-base -name "TUI Base" -desc "tui-base reference application"
