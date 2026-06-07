// Package config defines the types that allow any Model to contribute
// configurable fields to the application's Settings page.
//
// Usage pattern:
//
//  1. A component (page, nav, logger) implements [Configurable].
//  2. It returns a [Section] describing its fields with pointers into its own
//     state for live binding.
//  3. The router collects sections and passes them to [settings.New].
//  4. Settings builds one huh.Form group per section; Tab cycles naturally
//     through every field in every section with no custom pane-switch code.
package config

import (
	"fmt"

	huh "charm.land/huh/v2"
)

// FieldKind selects the huh widget type for a configurable field.
type FieldKind int

const (
	// FieldSelect renders a drop-down list of string choices.
	FieldSelect FieldKind = iota
	// FieldText renders a single-line free-form text input.
	FieldText
	// FieldFilePicker opens a file/directory browser.
	FieldFilePicker
)

// FieldDef describes one configurable value that a component exposes to the
// Settings page. Settings turns each FieldDef into the corresponding huh
// widget and keeps it live-bound to the component's own field via the Value
// pointer.
type FieldDef struct {
	// Key is a machine-readable identifier, used for JSON persistence.
	Key string
	// Title is the label rendered above the input.
	Title string
	// Description is an optional helper line rendered below the label when
	// the field is focused. It is muted/hidden when the field is blurred.
	Description string
	// Kind determines which huh widget is created.
	Kind FieldKind
	// Options is the ordered list of choices for FieldSelect widgets.
	Options []huh.Option[string]
	// Value must point to the string field in the owning model that backs
	// this setting. The pointer must remain valid for the lifetime of the
	// Settings page.
	Value *string
	// Height overrides the drop-down height for FieldSelect (0 = huh default).
	Height int
	// DirAllowed and FileAllowed configure FieldFilePicker.
	DirAllowed  bool
	FileAllowed bool
	// HideFunc, when non-nil, is called before rendering to decide whether
	// this field should be skipped. Returning true hides the field.
	HideFunc func() bool
	// Validate is an optional per-field validation function called when the
	// user moves focus away from the field.
	Validate func(string) error
	// Apply is an optional callback run when a field edit is submitted.
	// Use this to persist values outside tui-base settings storage.
	Apply func(string) error
}

// Section groups related FieldDefs under a named heading. Each Section
// becomes one huh.Group in the Settings form.
type Section struct {
	// Title is displayed as the section heading inside the Settings form.
	Title string
	// Fields is the ordered list of configurable values in this section.
	Fields []FieldDef
}

// Configurable can be implemented by any page model or router component that
// wants to expose its configuration in the Settings page. The router
// discovers Configurable implementations and passes their Sections to
// settings.New so they appear after the built-in sections.
type Configurable interface {
	ConfigSection() Section
}

// ── Field constructor helpers ─────────────────────────────────────────────
//
// These reduce the per-field boilerplate in consumer apps. Each function
// returns a FieldDef pre-configured for the most common patterns.

// BoolField returns a FieldSelect FieldDef with Enabled/Disabled options bound
// to a *string field that holds "true" or "false". The apply callback is called
// when the user submits the form; pass nil to skip the callback.
func BoolField(key, title, description string, value *string, apply func(string) error) FieldDef {
	return FieldDef{
		Key:         key,
		Title:       title,
		Description: description,
		Kind:        FieldSelect,
		Options: []huh.Option[string]{
			huh.NewOption("Enabled", "true"),
			huh.NewOption("Disabled", "false"),
		},
		Value: value,
		Validate: func(v string) error {
			if v == "true" || v == "false" {
				return nil
			}
			return fmt.Errorf("value must be true or false")
		},
		Apply: apply,
	}
}

// YesNoField is like BoolField but shows "Yes" / "No" instead of
// "Enabled" / "Disabled".
func YesNoField(key, title, description string, value *string, apply func(string) error) FieldDef {
	return FieldDef{
		Key:         key,
		Title:       title,
		Description: description,
		Kind:        FieldSelect,
		Options: []huh.Option[string]{
			huh.NewOption("Yes", "true"),
			huh.NewOption("No", "false"),
		},
		Value: value,
		Validate: func(v string) error {
			if v == "true" || v == "false" {
				return nil
			}
			return fmt.Errorf("value must be Yes or No")
		},
		Apply: apply,
	}
}

// EnumField returns a FieldSelect FieldDef with arbitrary label/value pairs.
func EnumField(key, title, description string, options []huh.Option[string], value *string, apply func(string) error) FieldDef {
	return FieldDef{
		Key:         key,
		Title:       title,
		Description: description,
		Kind:        FieldSelect,
		Options:     options,
		Value:       value,
		Apply:       apply,
	}
}

// TextField returns a FieldText FieldDef bound to a *string field with an
// optional validation function.
func TextField(key, title, description string, value *string, validate func(string) error, apply func(string) error) FieldDef {
	return FieldDef{
		Key:         key,
		Title:       title,
		Description: description,
		Kind:        FieldText,
		Value:       value,
		Validate:    validate,
		Apply:       apply,
	}
}
