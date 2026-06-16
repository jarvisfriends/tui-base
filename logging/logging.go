package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Subscriber is called for each log entry so other parts of the app (like the
// inspector) can display a brief-formatted view in the UI while the full
// timestamped output lives on disk.
type Subscriber func(level string, ts time.Time, msg string)

var (
	subsMu    sync.RWMutex
	subs      []Subscriber
	outFile   *os.File
	writeMu   sync.Mutex
	levelMu   sync.RWMutex
	appNameMu sync.RWMutex // guards logAppName

	// Log rotation state (all guarded by writeMu).
	logTarget   string            // path of the active log file
	curLogBytes int64             // bytes written to the active file so far
	maxLogBytes int64  = 10 << 20 // rotate once the file exceeds this; 0 disables

	logAppName = "tui-base" // overridden by SetAppName before InitFromSettings; guarded by appNameMu

	// minLevel is the minimum accepted log level: DEBUG=0, INFO=1, WARN=2, ERROR=3
	minLevel       = 3
	levelNameToInt = map[string]int{
		"DEBUG": 0,
		"INFO":  1,
		"WARN":  2,
		"ERROR": 3,
	}
	levelIntToName = map[int]string{
		0: "DEBUG",
		1: "INFO",
		2: "WARN",
		3: "ERROR",
	}
)

// SetAppName sets the application name used as the log file prefix and the
// default temp-log subdirectory. Call this before InitFromSettings.
func SetAppName(name string) {
	if name != "" {
		appNameMu.Lock()
		logAppName = name
		appNameMu.Unlock()
	}
}

// RegisterSubscriber registers a callback to receive log entries.
func RegisterSubscriber(s Subscriber) {
	subsMu.Lock()
	subs = append(subs, s)
	subsMu.Unlock()
}

func notify(level string, ts time.Time, msg string) {
	subsMu.RLock()
	defer subsMu.RUnlock()
	for _, s := range subs {
		// best-effort, run synchronously to keep ordering simple
		s(level, ts, msg)
	}
}

// InitFromSettings configures the global charm logger according to the
// settings model. It opens a file (either in a temp dir or a user-selected
// path) and routes logger output there. Returns the path of the log file on
// success.
func InitFromSettings(logOutputMode, logPath string) (string, error) {
	// determine output file
	var target string
	appNameMu.RLock()
	currentAppName := logAppName
	appNameMu.RUnlock()

	switch logOutputMode {
	case "dir":
		if logPath == "" {
			return "", fmt.Errorf("log directory not provided")
		}
		if err := os.MkdirAll(logPath, 0o755); err != nil {
			return "", err
		}
		target = filepath.Join(logPath, fmt.Sprintf("%s-%s.log", currentAppName, time.Now().Format("20060102-150405")))
	case "file":
		if logPath == "" {
			return "", fmt.Errorf("log file path not provided")
		}
		// ensure parent dir exists
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
			return "", err
		}
		target = logPath
	default:
		// default: temp dir
		dir := filepath.Join(os.TempDir(), currentAppName+"-logs")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		target = filepath.Join(dir, fmt.Sprintf("%s-%s.log", currentAppName, time.Now().Format("20060102-150405")))
	}

	f, err := os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	// Keep file handle; assign outFile/logTarget/curLogBytes inside writeMu so
	// concurrent readers of these fields (e.g. CurrentLogFile, rotateIfNeededLocked)
	// always see a consistent state.
	var initBytes int64
	if fi, statErr := f.Stat(); statErr == nil {
		initBytes = fi.Size()
	}
	// initial info (write directly to file)
	writeMu.Lock()
	outFile = f
	logTarget = target
	curLogBytes = initBytes
	n, _ := fmt.Fprintf(outFile, "%s [INFO] Logging initialized; file=%s\n", time.Now().Format(time.RFC3339), target)
	curLogBytes += int64(n)
	writeMu.Unlock()
	notify("INFO", time.Now(), fmt.Sprintf("Logging initialized; file=%s", target))
	return target, nil
}

// SetMaxLogBytes sets the size threshold (in bytes) at which the active log file
// is rotated to "<file>.1". A value <= 0 disables rotation. Safe to call at any
// time; it takes effect on the next write.
func SetMaxLogBytes(n int64) {
	writeMu.Lock()
	maxLogBytes = n
	writeMu.Unlock()
}

// rotateIfNeededLocked rotates the active log file when it exceeds maxLogBytes,
// keeping a single previous generation as "<file>.1". The caller MUST hold
// writeMu.
func rotateIfNeededLocked() {
	if maxLogBytes <= 0 || outFile == nil || logTarget == "" || curLogBytes < maxLogBytes {
		return
	}
	_ = outFile.Close()
	rotated := logTarget + ".1"
	_ = os.Remove(rotated) // discard the previous rotation, if any
	_ = os.Rename(logTarget, rotated)
	f, err := os.OpenFile(logTarget, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		outFile = nil // degrade to subscribers-only until the next InitFromSettings
		return
	}
	outFile = f
	curLogBytes = 0
}

// writeLog is the shared write path for every level helper: it enforces the
// minimum level, rotates the file when needed, writes the line, tracks the
// running byte count, and notifies subscribers.
func writeLog(level, format string, args ...any) {
	levelMu.RLock()
	enabled := levelNameToInt[level] >= minLevel
	levelMu.RUnlock()
	if !enabled {
		return
	}
	msg := fmt.Sprintf(format, args...)
	ts := time.Now()
	writeMu.Lock()
	if outFile != nil {
		rotateIfNeededLocked()
		n, _ := fmt.Fprintf(outFile, "%s [%s] %s\n", ts.Format(time.RFC3339), level, msg)
		curLogBytes += int64(n)
	}
	writeMu.Unlock()
	notify(level, ts, msg)
}

// Debugf logs a debug line and notifies subscribers.
func Debugf(format string, args ...any) { writeLog("DEBUG", format, args...) }

// Infof logs an info line and notifies subscribers.
func Infof(format string, args ...any) { writeLog("INFO", format, args...) }

// Errorf logs an error line and notifies subscribers.
func Errorf(format string, args ...any) { writeLog("ERROR", format, args...) }

// Warnf logs a warning line and notifies subscribers.
func Warnf(format string, args ...any) { writeLog("WARN", format, args...) }

// SetLevel sets the minimum log level. Valid values: DEBUG, INFO, WARN, ERROR.
func SetLevel(level string) error {
	level = strings.ToUpper(strings.TrimSpace(level))
	lvl, ok := levelNameToInt[level]
	if !ok {
		return fmt.Errorf("invalid log level: %s", level)
	}
	levelMu.Lock()
	minLevel = lvl
	levelMu.Unlock()
	Infof("Log level set to %s", level)
	return nil
}

// GetLevel returns the currently configured minimum log level as a string.
func GetLevel() string {
	levelMu.RLock()
	defer levelMu.RUnlock()
	if name, ok := levelIntToName[minLevel]; ok {
		return name
	}
	return "INFO"
}

// CurrentLogFile returns the path to the currently open log file, if any.
func CurrentLogFile() string {
	writeMu.Lock()
	path := logTarget
	writeMu.Unlock()
	return path
}
