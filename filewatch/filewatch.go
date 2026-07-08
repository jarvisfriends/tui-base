// Package filewatch bridges fsnotify file-system events into Bubble Tea
// messages so views can live-reload when files change on disk (FW-1).
//
// The watcher always watches the target's parent directory and filters events
// down to the requested path. Watching the directory — not the file — is what
// makes atomic writers (temp file + rename, e.g. common.WriteFileAtomic) and
// editors that replace the file visible: an inode-level watch dies with the
// old inode on the first rename.
//
// Usage inside a Bubble Tea model:
//
//	w, err := filewatch.New(path)          // in the constructor
//	cmd := w.Next()                        // arm in Init (or after New)
//	case filewatch.Event:                  // in Update
//	    ... react ...
//	    return m, m.watcher.Next()         // re-arm for the next change
//	case filewatch.ErrorMsg:               // watcher failed; do not re-arm
//	w.Stop()                               // when the program exits
//
// Next blocks in a tea.Cmd goroutine, so the UI stays responsive. Bursts of
// events (editors often emit create+write+chmod within milliseconds) are
// coalesced by a quiet-period debounce before the Event is delivered.
package filewatch

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/fsnotify/fsnotify"
)

// Event reports that the watched path changed on disk. It is delivered as a
// tea.Msg by the command returned from Next.
type Event struct {
	// Path is the cleaned absolute path that changed.
	Path string
	// Op describes the final operation observed in the debounce window
	// (fsnotify's string form, e.g. "WRITE", "CREATE", "RENAME").
	Op string
}

// ErrorMsg reports a watcher failure. After receiving it the watcher is dead:
// call Stop and do not re-arm Next.
type ErrorMsg struct {
	Path string
	Err  error
}

// DefaultDebounce is the quiet period used to coalesce event bursts when no
// override is supplied via WithDebounce.
const DefaultDebounce = 100 * time.Millisecond

// Option customizes a Watcher created by New.
type Option func(*Watcher)

// WithDebounce overrides the quiet period used to coalesce event bursts.
// Values <= 0 disable debouncing (every raw event is delivered).
func WithDebounce(d time.Duration) Option {
	return func(w *Watcher) { w.debounce = d }
}

// Watcher delivers file-change notifications for one file or directory as
// Bubble Tea messages. Create it with New, arm it with Next, and release the
// OS watch with Stop. All methods are safe for concurrent use.
type Watcher struct {
	fw       *fsnotify.Watcher
	path     string // cleaned absolute target
	filter   string // exact file path to match; "" = deliver everything in dir
	debounce time.Duration
}

// New creates a watcher for path. The path may be a file (existing or not —
// its parent directory must be creatable) or an existing directory. For a
// file, only events for that exact path are delivered; for a directory, every
// change inside it is delivered.
func New(path string, opts ...Option) (*Watcher, error) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	watchDir := abs
	filter := ""
	if info, statErr := os.Stat(abs); statErr != nil || !info.IsDir() {
		// Treat as a file target: watch the parent so atomic renames and
		// not-yet-created files are still observed.
		watchDir = filepath.Dir(abs)
		filter = abs
		if mkErr := os.MkdirAll(watchDir, 0o750); mkErr != nil {
			return nil, mkErr
		}
	}

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := fw.Add(watchDir); err != nil {
		_ = fw.Close()
		return nil, err
	}

	w := &Watcher{fw: fw, path: abs, filter: filter, debounce: DefaultDebounce}
	for _, opt := range opts {
		opt(w)
	}
	return w, nil
}

// Path returns the cleaned absolute path this watcher was created for.
func (w *Watcher) Path() string { return w.path }

// Next returns a command that blocks until the next matching change and
// delivers it as an Event (or ErrorMsg on watcher failure). It returns nil
// after Stop. Re-arm by calling Next again from your Update handler.
func (w *Watcher) Next() tea.Cmd {
	return func() tea.Msg {
		ev, ok := w.awaitMatch()
		if !ok {
			return nil // stopped
		}
		if watchErr, isErr := ev.(error); isErr {
			return ErrorMsg{Path: w.path, Err: watchErr}
		}
		first, isEvent := ev.(fsnotify.Event)
		if !isEvent {
			return nil
		}
		last := w.drainQuiet(first)
		return Event{Path: w.path, Op: last.Op.String()}
	}
}

// awaitMatch blocks until a matching fsnotify event or an error arrives.
// The bool result is false when the watcher was closed.
func (w *Watcher) awaitMatch() (any, bool) {
	for {
		select {
		case ev, ok := <-w.fw.Events:
			if !ok {
				return nil, false
			}
			if w.matches(ev) {
				return ev, true
			}
		case err, ok := <-w.fw.Errors:
			if !ok {
				return nil, false
			}
			return err, true
		}
	}
}

// drainQuiet coalesces the burst following first: it keeps consuming matching
// events until the stream has been quiet for the debounce window, returning
// the last event observed.
func (w *Watcher) drainQuiet(first fsnotify.Event) fsnotify.Event {
	if w.debounce <= 0 {
		return first
	}
	last := first
	timer := time.NewTimer(w.debounce)
	defer timer.Stop()
	for {
		select {
		case ev, ok := <-w.fw.Events:
			if !ok {
				return last
			}
			if w.matches(ev) {
				last = ev
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(w.debounce)
			}
		case <-timer.C:
			return last
		}
	}
}

// matches reports whether ev is for the watched target.
func (w *Watcher) matches(ev fsnotify.Event) bool {
	if w.filter == "" {
		return true
	}
	evPath, err := filepath.Abs(filepath.Clean(ev.Name))
	if err != nil {
		return false
	}
	return evPath == w.filter
}

// Stop releases the OS watch. Pending and future Next commands complete with
// a nil message. Stop is idempotent.
func (w *Watcher) Stop() error {
	err := w.fw.Close()
	if errors.Is(err, fsnotify.ErrClosed) {
		return nil
	}
	return err
}
