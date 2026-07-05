// Package inspector is the built-in runtime observability overlay (Ctrl+D):
// tabbed views for Go runtime stats, input telemetry, disks, terminal/theme
// diagnostics, accessibility (CVD) checks, an intercepted-message log, and
// debug settings including pprof capture.
//
// The router feeds it every non-key message even while hidden so the log and
// telemetry stay current (ADR-006). Tab cycling follows the application's
// next/previous page bindings via SetNavKeys. Applications add custom tabs by
// implementing MetricsProvider and calling router.RegisterInspectorTab — see
// docs/inspector-extensions.md; providers are started when the inspector
// opens and stopped when it closes.
package inspector
