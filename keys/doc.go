// Package keys defines AppKeyMap, the application-wide key bindings (quit,
// page cycling, settings, nav/status toggles, inspector) with bubbles/help
// integration and user rebinding support.
//
// Bindings live in the struct per ADR-011: help methods never construct
// bindings inline, and vim fallback keys (j/k/h/l) are prohibited in favor of
// the standardized arrows and named bindings. ApplyCustomizations applies
// user overrides persisted by the Settings page; BindingDefs feeds the
// rebinding UI.
package keys
