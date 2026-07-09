package settings

import "github.com/jarvisfriends/tui-base/winterm"

// detectDefaultTerminal reads the machine's default-terminal delegation via
// the winterm package and maps it to the option key used by the settings UI.
// It returns "" (with a nil error) when the state is unrecognized, and an
// error off Windows — callers treat both as "leave the current value alone".
func detectDefaultTerminal() (string, error) {
	d, err := winterm.Detect()
	if err != nil {
		return "", err
	}
	switch d {
	case winterm.LetWindowsDecide:
		return defTerminalLetWindows, nil
	case winterm.LegacyConsole:
		return defTerminalClassic, nil
	case winterm.WindowsTerminal:
		return defTerminalModern, nil
	case winterm.Unknown:
		return "", nil
	default:
		return "", nil
	}
}

// applyTerminalSetting writes the delegation selected in the settings UI.
// Unknown option keys are ignored so a stale persisted value can never write
// garbage to the registry.
func applyTerminalSetting(option string) error {
	switch option {
	case defTerminalLetWindows:
		return winterm.Set(winterm.LetWindowsDecide)
	case defTerminalClassic:
		return winterm.Set(winterm.LegacyConsole)
	case defTerminalModern:
		return winterm.Set(winterm.WindowsTerminal)
	default:
		return nil
	}
}
