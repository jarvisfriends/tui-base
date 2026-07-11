package router

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jarvisfriends/snap/notifications"
	"github.com/jarvisfriends/tui-base/filewatch"
	log "github.com/jarvisfriends/tui-base/logging"
	"github.com/jarvisfriends/tui-base/pages/settings"
)

// startSettingsWatch creates the settings-file watcher when
// Options.WatchSettingsFile is set. Failure is non-fatal: the app runs
// without live reload and the reason is logged.
func (m *RouterModel) startSettingsWatch() {
	w, err := filewatch.New(settings.FilePath())
	if err != nil {
		log.Warnf("settings file watch disabled: %v", err)
		return
	}
	m.settingsWatcher = w
}

// settingsWatchInit returns the command that arms the watcher, or nil when
// watching is disabled.
func (m *RouterModel) settingsWatchInit() tea.Cmd {
	if m.settingsWatcher == nil {
		return nil
	}
	return m.settingsWatcher.Next()
}

// handleSettingsFileEvent reloads settings after an on-disk change and
// notifies the user when the change came from outside the app (the app's own
// saves reload as a no-op and stay silent). It always re-arms the watcher.
func (m *RouterModel) handleSettingsFileEvent() tea.Cmd {
	cmds := []tea.Cmd{m.settingsWatchInit()}
	changed, themeCmd := m.settingsPage.ReloadFromDisk()
	if changed {
		log.Infof("settings file changed on disk; reloaded")
		if themeCmd != nil {
			cmds = append(cmds, themeCmd)
		}
		if m.notifMgr != nil {
			_, notifCmd := m.notifMgr.Add(
				"Settings reloaded from disk",
				notifications.SeverityInfo,
				notifications.SeverityInfo.DefaultTTL(),
			)
			if notifCmd != nil {
				cmds = append(cmds, notifCmd)
			}
		}
	}
	return tea.Batch(cmds...)
}

// handleSettingsFileError logs a dead watcher. The watcher is not re-armed —
// filewatch.ErrorMsg means the underlying OS watch failed.
func (m *RouterModel) handleSettingsFileError(msg filewatch.ErrorMsg) {
	log.Warnf("settings file watch stopped: %v", msg.Err)
	m.stopSettingsWatch()
}

// stopSettingsWatch releases the OS watch. Safe to call when disabled.
func (m *RouterModel) stopSettingsWatch() {
	if m.settingsWatcher == nil {
		return
	}
	_ = m.settingsWatcher.Stop()
	m.settingsWatcher = nil
}

// Close releases resources the router holds outside the Bubble Tea loop
// (currently the settings-file watcher). Call it after Program.Run returns;
// tuibase.RunContext does this automatically.
func (m *RouterModel) Close() {
	m.stopSettingsWatch()
}
