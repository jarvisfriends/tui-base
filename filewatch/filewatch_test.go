// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package filewatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// runNext executes the Next command on a goroutine and returns the delivered
// message (or nil) via the channel.
func runNext(w *Watcher) <-chan tea.Msg {
	ch := make(chan tea.Msg, 1)
	cmd := w.Next()
	go func() { ch <- cmd() }()
	return ch
}

func waitMsg(t *testing.T, ch <-chan tea.Msg) tea.Msg {
	t.Helper()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for filewatch message")
		return nil
	}
}

func TestFileWriteDeliversEvent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	w, err := New(target, WithDebounce(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	ch := runNext(w)
	time.Sleep(50 * time.Millisecond) // let the watch arm before writing
	if err := os.WriteFile(target, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatal(err)
	}

	msg := waitMsg(t, ch)
	ev, ok := msg.(Event)
	if !ok {
		t.Fatalf("got %T, want Event", msg)
	}
	if ev.Path != w.Path() {
		t.Errorf("Event.Path = %q, want %q", ev.Path, w.Path())
	}
}

func TestAtomicRenameDeliversEvent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	w, err := New(target, WithDebounce(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	ch := runNext(w)
	time.Sleep(50 * time.Millisecond)

	// The WriteFileAtomic pattern: write a temp file, rename over the target.
	tmp := filepath.Join(dir, "settings.json.tmp")
	if err := os.WriteFile(tmp, []byte(`{"b":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, target); err != nil {
		t.Fatal(err)
	}

	if _, ok := waitMsg(t, ch).(Event); !ok {
		t.Fatal("atomic rename over the target was not observed")
	}
}

func TestOtherFilesAreFiltered(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "settings.json")
	other := filepath.Join(dir, "unrelated.txt")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	w, err := New(target, WithDebounce(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	ch := runNext(w)
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(other, []byte("noise"), 0o600); err != nil {
		t.Fatal(err)
	}

	select {
	case msg := <-ch:
		t.Fatalf("unrelated file delivered %v", msg)
	case <-time.After(300 * time.Millisecond):
		// expected: no event for the unrelated file
	}

	// The watcher must still be armed for the real target.
	if err := os.WriteFile(target, []byte(`{"c":3}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := waitMsg(t, ch).(Event); !ok {
		t.Fatal("target write after unrelated noise was not observed")
	}
}

func TestNotYetCreatedFileIsObserved(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "later.json")

	w, err := New(target, WithDebounce(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	ch := runNext(w)
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := waitMsg(t, ch).(Event); !ok {
		t.Fatal("creation of the watched file was not observed")
	}
}

func TestBurstIsCoalesced(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	w, err := New(target, WithDebounce(150*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	ch := runNext(w)
	time.Sleep(50 * time.Millisecond)
	for i := range 5 {
		if err := os.WriteFile(target, []byte{byte('0' + i)}, 0o600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, ok := waitMsg(t, ch).(Event); !ok {
		t.Fatal("burst did not deliver an event")
	}

	// The burst must produce exactly one Event: a re-armed Next stays quiet.
	ch2 := runNext(w)
	select {
	case msg := <-ch2:
		if msg != nil {
			t.Fatalf("burst produced a second message: %v", msg)
		}
	case <-time.After(400 * time.Millisecond):
		// expected: coalesced into the first Event
	}
	_ = w.Stop()
}

func TestStopUnblocksNext(t *testing.T) {
	dir := t.TempDir()
	w, err := New(filepath.Join(dir, "x.json"))
	if err != nil {
		t.Fatal(err)
	}

	ch := runNext(w)
	time.Sleep(30 * time.Millisecond)
	if err := w.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if msg := waitMsg(t, ch); msg != nil {
		t.Fatalf("stopped watcher delivered %v, want nil", msg)
	}
	if err := w.Stop(); err != nil {
		t.Fatalf("second Stop must be idempotent, got %v", err)
	}
}

func TestWatchDirectory(t *testing.T) {
	dir := t.TempDir()
	w, err := New(dir, WithDebounce(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()

	ch := runNext(w)
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "anything.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := waitMsg(t, ch).(Event); !ok {
		t.Fatal("directory watch did not observe a new file")
	}
}
