package config

import (
	"fmt"
	"testing"

	huh "charm.land/huh/v2"
)

type stubConfigurable struct {
	section Section
}

func (s stubConfigurable) ConfigSection() Section { return s.section }

func TestFieldDefValueRoundTrip(t *testing.T) {
	t.Parallel()

	value := "INFO"
	field := FieldDef{
		Key:   "log-level",
		Title: "Log Level",
		Kind:  FieldSelect,
		Value: &value,
	}

	*field.Value = "DEBUG"
	if value != "DEBUG" {
		t.Fatalf("backing value = %q; want %q", value, "DEBUG")
	}
	if field.Kind != FieldSelect {
		t.Fatalf("field kind = %v; want %v", field.Kind, FieldSelect)
	}
}

func TestConfigurableInterfaceProvidesSection(t *testing.T) {
	t.Parallel()

	section := Section{Title: "Logging"}
	var configurable Configurable = stubConfigurable{section: section}
	if got := configurable.ConfigSection(); got.Title != section.Title {
		t.Fatalf("section title = %q; want %q", got.Title, section.Title)
	}
}

func TestBoolFieldConstructor(t *testing.T) {
	t.Parallel()
	val := "true"
	applied := ""
	apply := func(v string) error {
		applied = v
		return nil
	}

	field := BoolField("bkey", "btitle", "bdesc", &val, apply)
	if field.Key != "bkey" || field.Title != "btitle" || field.Description != "bdesc" {
		t.Errorf("incorrect fields in BoolField: %+v", field)
	}
	if err := field.Validate("true"); err != nil {
		t.Errorf("expected valid for true: %v", err)
	}
	if err := field.Validate("false"); err != nil {
		t.Errorf("expected valid for false: %v", err)
	}
	if err := field.Validate("invalid"); err == nil {
		t.Errorf("expected invalid for non-bool string")
	}
	_ = field.Apply("true")
	if applied != "true" {
		t.Errorf("expected apply callback to run; got %q", applied)
	}
}

func TestYesNoFieldConstructor(t *testing.T) {
	t.Parallel()
	val := "false"
	applied := ""
	apply := func(v string) error {
		applied = v
		return nil
	}

	field := YesNoField("ynkey", "yntitle", "yndesc", &val, apply)
	if field.Key != "ynkey" || field.Title != "yntitle" || field.Description != "yndesc" {
		t.Errorf("incorrect fields in YesNoField: %+v", field)
	}
	if err := field.Validate("true"); err != nil {
		t.Errorf("expected valid for true: %v", err)
	}
	if err := field.Validate("false"); err != nil {
		t.Errorf("expected valid for false: %v", err)
	}
	if err := field.Validate("invalid"); err == nil {
		t.Errorf("expected invalid for non-bool string")
	}
	_ = field.Apply("false")
	if applied != "false" {
		t.Errorf("expected apply callback to run; got %q", applied)
	}
}

func TestEnumFieldConstructor(t *testing.T) {
	t.Parallel()
	val := "val1"
	applied := ""
	apply := func(v string) error {
		applied = v
		return nil
	}

	options := []huh.Option[string]{
		huh.NewOption("Option 1", "val1"),
		huh.NewOption("Option 2", "val2"),
	}
	field := EnumField("ekey", "etitle", "edesc", options, &val, apply)
	if field.Key != "ekey" || field.Title != "etitle" || field.Description != "edesc" {
		t.Errorf("incorrect fields in EnumField: %+v", field)
	}
	if len(field.Options) != 2 || field.Options[0].Value != "val1" {
		t.Errorf("enum options not correctly populated")
	}
	_ = field.Apply("val2")
	if applied != "val2" {
		t.Errorf("expected apply callback to run; got %q", applied)
	}
}

func TestTextFieldConstructor(t *testing.T) {
	t.Parallel()
	val := "textval"
	applied := ""
	apply := func(v string) error {
		applied = v
		return nil
	}
	validate := func(v string) error {
		if len(v) < 3 {
			return fmt.Errorf("too short")
		}
		return nil
	}

	field := TextField("tkey", "ttitle", "tdesc", &val, validate, apply)
	if field.Key != "tkey" || field.Title != "ttitle" || field.Description != "tdesc" {
		t.Errorf("incorrect fields in TextField: %+v", field)
	}
	if err := field.Validate("ok"); err == nil {
		t.Errorf("expected validation to fail for 'ok'")
	}
	if err := field.Validate("valid"); err != nil {
		t.Errorf("expected validation to pass for 'valid'")
	}
	_ = field.Apply("newval")
	if applied != "newval" {
		t.Errorf("expected apply callback to run; got %q", applied)
	}
}
