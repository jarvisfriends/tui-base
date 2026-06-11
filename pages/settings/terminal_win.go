//go:build windows

package settings

import (
	"github.com/jarvisfriends/tui-base/logging"
	"golang.org/x/sys/windows/registry"
)

// detectDefaultTerminal reads the per-user Console\%Startup registry keys and
// returns one of the internal option keys used by the UI: "let_windows",
// "classic", or "modern". Returns empty string when unable to determine.
func detectDefaultTerminal() (string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Console\\%%Startup`, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return "let_windows", nil
		}
		return "", err
	}
	defer func() {
		if cerr := k.Close(); cerr != nil {
			logging.Errorf("settings: registry key close failed: %v", cerr)
		}
	}()

	consoleGUID, _, err := k.GetStringValue("DelegationConsole")
	if err != nil {
		consoleGUID = "{00000000-0000-0000-0000-000000000000}"
	}
	terminalGUID, _, err := k.GetStringValue("DelegationTerminal")
	if err != nil {
		terminalGUID = "{00000000-0000-0000-0000-000000000000}"
	}

	switch {
	case consoleGUID == "{00000000-0000-0000-0000-000000000000}" && terminalGUID == "{00000000-0000-0000-0000-000000000000}":
		return "let_windows", nil
	case consoleGUID == "{B23D10C0-E52E-411E-9D5B-C09FDF709C7D}" && terminalGUID == "{B23D10C0-E52E-411E-9D5B-C09FDF709C7D}":
		return "classic", nil
	case consoleGUID == "{2EACA947-7F5F-4CFA-BA87-8F7FBEEFBE69}" && terminalGUID == "{E12CFF52-A866-4C77-9A90-F570A7AA2C6B}":
		return "modern", nil
	default:
		return "", nil
	}
}

// applyTerminalSetting writes DelegationConsole and DelegationTerminal for the
// current user. This performs the same registry commit as the tmp/default_terminal
// example.
func applyTerminalSetting(consoleGUID, terminalGUID string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Console\\%%Startup`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := k.Close(); cerr != nil {
			logging.Errorf("settings: registry key close failed: %v", cerr)
		}
	}()

	if err := k.SetStringValue("DelegationConsole", consoleGUID); err != nil {
		return err
	}
	return k.SetStringValue("DelegationTerminal", terminalGUID)
}
