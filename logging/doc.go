// Package logging is tui-base's runtime logger: leveled Debugf/Infof/Warnf/
// Errorf functions writing to a rotating file (or the system temp directory),
// with a subscriber fan-out that feeds the in-app inspector's Log tab.
//
// Use it for all logging inside models — never fmt.Printf or log.Println,
// which would corrupt the terminal UI. Subscribers are invoked outside the
// registration lock, so a subscriber may itself log or register safely. The
// level and destination are runtime-configurable from the Settings page.
package logging
