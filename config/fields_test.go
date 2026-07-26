// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package config

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// TestPickerFieldConstructors pins the Kind and browse permissions each
// picker constructor encodes: multi-file rows may pick files and dirs,
// multi-dir rows are directories only.
func TestPickerFieldConstructors(t *testing.T) {
	t.Parallel()

	v := "a;b"
	applied := ""
	f := MultiFilePickerField("k", "Files", "desc", &v, func(s string) error {
		applied = s
		return nil
	})
	if f.Kind != FieldMultiFilePicker || !f.FileAllowed || !f.DirAllowed {
		t.Fatalf("MultiFilePickerField = kind %v file=%v dir=%v", f.Kind, f.FileAllowed, f.DirAllowed)
	}
	if f.Key != "k" || f.Title != "Files" || f.Value != &v {
		t.Fatalf("MultiFilePickerField wiring: %+v", f)
	}
	if err := f.Apply("x;y"); err != nil || applied != "x;y" {
		t.Fatalf("Apply passthrough: err=%v applied=%q", err, applied)
	}

	d := MultiDirPickerField("k2", "Dirs", "desc", &v, nil)
	if d.Kind != FieldMultiFilePicker || d.FileAllowed || !d.DirAllowed {
		t.Fatalf("MultiDirPickerField = kind %v file=%v dir=%v", d.Kind, d.FileAllowed, d.DirAllowed)
	}
}

// TestTypedFieldConstructors pins DateField, DurationField, and CustomField.
func TestTypedFieldConstructors(t *testing.T) {
	t.Parallel()

	when := time.Now()
	df := DateField("d", "Date", "desc", &when, nil)
	if df.Kind != FieldDate || df.Value != &when {
		t.Fatalf("DateField = %+v", df)
	}

	dur := time.Minute
	du := DurationField("t", "Timeout", "desc", &dur, nil)
	if du.Kind != FieldDuration || du.Value != &dur {
		t.Fatalf("DurationField = %+v", du)
	}

	built := 0
	cf := CustomField[string]("c", "Custom", "desc", "shown text", func() tea.Model {
		built++
		return nil
	})
	if cf.Kind != FieldCustom || cf.CustomFieldText != "shown text" {
		t.Fatalf("CustomField = %+v", cf)
	}
	if cf.CustomModelBuilder == nil {
		t.Fatal("CustomField should carry the builder")
	}
	_ = cf.CustomModelBuilder()
	if built != 1 {
		t.Fatal("builder not invoked")
	}
}
