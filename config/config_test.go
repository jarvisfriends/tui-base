package config

import "testing"

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
