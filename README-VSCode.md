# VS Code workspace helper

Quick notes for running and testing this project inside VS Code.

Run the TUI app
- Use the Debug configuration `Run TUI (integrated terminal)` or `Run TUI (external terminal)`.
- Or run the task: `Terminal > Run Task... > go: run TUI`.

Run tests and coverage
- Run `Terminal > Run Task... > go: test` to run unit tests.
- Run `Terminal > Run Task... > go: test (race)` to run tests with the race detector.
- Run `Terminal > Run Task... > go: coverage` to run the included coverage script which writes `coverage.out` and `coverage.txt`.
- Open `coverage.html` after `go: coverage` to inspect the coverage HTML report.

Profiling and performance
- Use `go test -bench . -benchmem ./...` (task: `go: bench`) to run benchmarks and collect alloc stats.
- For CPU/memory profiles, add targeted tests that call `testing.CPUProfile` / `pprof` and run them locally.

Terminal note
- This workspace sets the default integrated terminal to `Git Bash` on Windows; change in `.vscode/settings.json` if needed.

TinyGo attempts
- Run `bash tools/tinygo_build.sh` to attempt TinyGo builds (writes `dist/tinygo_build.log`).
- Result: TinyGo was present in this environment but the build failed when compiling `charm.land/bubbletea/v2` and other dependencies. Errors include undefined platform-specific symbols such as `listenForResize`, `suspendSupported`, and `initInput`, and missing TinyGo targets for native builds. See `dist/tinygo_build.log` for full output.
- Recommendation: Building this TUI with TinyGo requires either using TinyGo-compatible replacements for terminal libraries or isolating/removing platform-specific code paths (e.g., syscall/tty handling) in dependencies. For now, TinyGo builds are logged but not integrated into the goreleaser pipeline.
