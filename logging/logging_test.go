package logging

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func resetLoggerState(t *testing.T) {
	t.Helper()

	oldSubs := subs
	oldOutFile := outFile
	oldMinLevel := minLevel
	oldAppName := logAppName

	subs = nil
	outFile = nil
	minLevel = 1
	logAppName = "tui-base"

	t.Cleanup(func() {
		if outFile != nil && outFile != oldOutFile {
			_ = outFile.Close()
		}
		subs = oldSubs
		outFile = oldOutFile
		minLevel = oldMinLevel
		logAppName = oldAppName
	})
}

func TestInitFromSettingsFileModeSetsCurrentLogFile(t *testing.T) {
	resetLoggerState(t)

	path := filepath.Join(t.TempDir(), "test.log")
	got, err := InitFromSettings("file", path)
	if err != nil {
		t.Fatalf("InitFromSettings returned error: %v", err)
	}
	t.Cleanup(func() {
		if outFile != nil {
			_ = outFile.Close()
			outFile = nil
		}
	})
	if got != path {
		t.Fatalf("InitFromSettings path = %q; want %q", got, path)
	}
	if current := CurrentLogFile(); current != path {
		t.Fatalf("CurrentLogFile() = %q; want %q", current, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected initialized log file to contain data")
	}
}

func TestSetLevelFiltersSubscriberNotifications(t *testing.T) {
	resetLoggerState(t)

	path := filepath.Join(t.TempDir(), "events.log")
	if _, err := InitFromSettings("file", path); err != nil {
		t.Fatalf("InitFromSettings returned error: %v", err)
	}
	t.Cleanup(func() {
		if outFile != nil {
			_ = outFile.Close()
			outFile = nil
		}
	})

	type event struct {
		level string
		msg   string
		ts    time.Time
	}
	var events []event
	RegisterSubscriber(func(level string, ts time.Time, msg string) {
		events = append(events, event{level: level, ts: ts, msg: msg})
	})

	if err := SetLevel("ERROR"); err != nil {
		t.Fatalf("SetLevel returned error: %v", err)
	}
	Debugf("debug hidden")
	Warnf("warn hidden")
	Errorf("error visible")

	if got := GetLevel(); got != "ERROR" {
		t.Fatalf("GetLevel() = %q; want %q", got, "ERROR")
	}
	if len(events) != 1 {
		t.Fatalf("subscriber event count = %d; want 1", len(events))
	}
	if events[0].level != "ERROR" || events[0].msg != "error visible" {
		t.Fatalf("subscriber event = %+v; want ERROR/error visible", events[0])
	}
	if events[0].ts.IsZero() {
		t.Fatal("expected subscriber event timestamp to be set")
	}
}

func TestSetLevelRejectsInvalidValue(t *testing.T) {
	resetLoggerState(t)

	if err := SetLevel("bogus"); err == nil {
		t.Fatal("expected invalid level error")
	}
}
