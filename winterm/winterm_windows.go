//go:build windows

package winterm

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

// startupKeyPath is the per-user key holding the default-terminal delegation.
// The subkey really is named "%%Startup" — the percent signs are part of the
// key name, not an environment-variable reference.
const startupKeyPath = `Console\%%Startup`

// Detect reads the current default-terminal delegation for this user. A
// missing key or missing values means the OS default ([LetWindowsDecide]); a
// GUID pair this package does not recognize reports [Unknown] with a nil
// error.
func Detect() (Delegation, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, startupKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return LetWindowsDecide, nil
		}
		return Unknown, err
	}
	defer func() { _ = k.Close() }() // read-only handle; close cannot lose data

	consoleGUID, _, err := k.GetStringValue("DelegationConsole")
	if err != nil {
		consoleGUID = guidLetWindows
	}
	terminalGUID, _, err := k.GetStringValue("DelegationTerminal")
	if err != nil {
		terminalGUID = guidLetWindows
	}
	return delegationFromGUIDs(consoleGUID, terminalGUID), nil
}

// Set writes the default-terminal delegation for this user. It rejects
// [Unknown]. The change affects console sessions started after the write;
// already-open windows keep their current host.
func Set(d Delegation) (err error) {
	consoleGUID, terminalGUID, ok := guidsForDelegation(d)
	if !ok {
		return errors.New("winterm: cannot set an unknown delegation")
	}

	k, _, err := registry.CreateKey(registry.CURRENT_USER, startupKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, k.Close()) }()

	if err := k.SetStringValue("DelegationConsole", consoleGUID); err != nil {
		return err
	}
	return k.SetStringValue("DelegationTerminal", terminalGUID)
}
