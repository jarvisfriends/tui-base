// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package filewatch

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestNewFailsWhenParentDirIsAFile(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	// The target's parent "directory" is a regular file, so the MkdirAll that
	// prepares the watch directory must fail.
	if _, err := New(filepath.Join(blocker, "sub", "settings.json")); err == nil {
		t.Fatal("New should fail when the watch directory cannot be created")
	}
}

func TestWatcherErrorDeliversErrorMsg(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "settings.json")
	w, err := New(target)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	ch := runNext(w)
	// Injecting into the fsnotify error channel simulates a dead OS watch;
	// the unbuffered send completes exactly when Next's awaitMatch receives.
	w.fw.Errors <- errors.New("watch backend failed")

	msg := waitMsg(t, ch)
	em, ok := msg.(ErrorMsg)
	if !ok {
		t.Fatalf("got %T, want ErrorMsg", msg)
	}
	if em.Path != w.Path() {
		t.Errorf("ErrorMsg.Path = %q, want %q", em.Path, w.Path())
	}
	if em.Err == nil {
		t.Error("ErrorMsg.Err should carry the watcher error")
	}
}

func TestZeroDebounceDeliversFirstEventImmediately(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	w, err := New(target, WithDebounce(0))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	ch := runNext(w)
	time.Sleep(50 * time.Millisecond) // let the watch arm before writing
	if err := os.WriteFile(target, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := waitMsg(t, ch).(Event); !ok {
		t.Fatal("zero-debounce watcher did not deliver the event")
	}
}

func TestDrainQuietReturnsLastWhenEventsClose(t *testing.T) {
	dir := t.TempDir()
	w, err := New(filepath.Join(dir, "x.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Stop(); err != nil {
		t.Fatal(err)
	}

	// With the watcher closed the Events channel is closed, so the debounce
	// drain must return the event it already has instead of blocking.
	first := fsnotify.Event{Name: w.Path(), Op: fsnotify.Write}
	if got := w.drainQuiet(first); got.Op != fsnotify.Write {
		t.Fatalf("drainQuiet returned %v, want the first event", got)
	}
}

func TestAwaitMatchReportsClosedChannels(t *testing.T) {
	dir := t.TempDir()
	w, err := New(filepath.Join(dir, "x.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Stop(); err != nil {
		t.Fatal(err)
	}

	// Both fsnotify channels are closed now; the select picks either arm, so
	// repeat until both closed-channel returns have been observed.
	for range 64 {
		if _, ok := w.awaitMatch(); ok {
			t.Fatal("awaitMatch on a stopped watcher must report ok=false")
		}
	}
}
