// Package winterm reads and writes the Windows *default terminal* delegation —
// the per-user setting that decides whether console applications open in the
// legacy console host (conhost) or in Windows Terminal.
//
// There is no supported Windows API for this setting: Windows Terminal's own
// settings UI writes the DelegationConsole/DelegationTerminal registry values
// under HKCU\Console\%%Startup, and this package does exactly the same. It
// exists because that setting is known to be reset on some machines (breaking
// the truecolor/mouse/styling features Charm v2 apps rely on), so tui-base apps
// can surface it in their settings UI — see the "Default Terminal" item on the
// built-in settings page — or repair it programmatically.
//
// Detect and Set are Windows-only; on other platforms they return
// [errors.ErrUnsupported]. Changes affect only console sessions started after
// the write. The related runtime guard is
// [github.com/jarvisfriends/tui-base/router.MaybeRelaunchInWindowsTerminal],
// which moves an already-running app out of a legacy console without touching
// machine state.
package winterm

// Delegation identifies which host Windows hands new console sessions to.
type Delegation int

const (
	// Unknown means the registry holds a GUID pair this package does not
	// recognize (e.g. a preview build or a third-party host). Detect reports
	// it; Set rejects it.
	Unknown Delegation = iota
	// LetWindowsDecide is the OS default ("Let Windows decide" in the Windows
	// Terminal settings UI): the zero GUID in both values, or no values at all.
	LetWindowsDecide
	// LegacyConsole is the classic conhost host.
	LegacyConsole
	// WindowsTerminal is the modern Windows Terminal host.
	WindowsTerminal
)

// delegationUnknownName is the display name for Unknown and any out-of-range
// value.
const delegationUnknownName = "unknown"

// String returns a human-readable name for the delegation value.
func (d Delegation) String() string {
	switch d {
	case LetWindowsDecide:
		return "let Windows decide"
	case LegacyConsole:
		return "legacy console (conhost)"
	case WindowsTerminal:
		return "Windows Terminal"
	case Unknown:
		return delegationUnknownName
	default:
		return delegationUnknownName
	}
}

// Well-known delegation CLSIDs. These are the exact values the Windows
// Terminal settings UI writes.
const (
	guidLetWindows = "{00000000-0000-0000-0000-000000000000}"
	guidConhost    = "{B23D10C0-E52E-411E-9D5B-C09FDF709C7D}"
	guidWTConsole  = "{2EACA947-7F5F-4CFA-BA87-8F7FBEEFBE69}"
	guidWTTerminal = "{E12CFF52-A866-4C77-9A90-F570A7AA2C6B}"
)

// delegationFromGUIDs maps a DelegationConsole/DelegationTerminal value pair to
// the Delegation it represents. Missing values are passed as the zero GUID.
func delegationFromGUIDs(consoleGUID, terminalGUID string) Delegation {
	switch {
	case consoleGUID == guidLetWindows && terminalGUID == guidLetWindows:
		return LetWindowsDecide
	case consoleGUID == guidConhost && terminalGUID == guidConhost:
		return LegacyConsole
	case consoleGUID == guidWTConsole && terminalGUID == guidWTTerminal:
		return WindowsTerminal
	default:
		return Unknown
	}
}

// guidsForDelegation returns the registry value pair for d, with ok=false when
// d has no defined pair (Unknown).
func guidsForDelegation(d Delegation) (consoleGUID, terminalGUID string, ok bool) {
	switch d {
	case LetWindowsDecide:
		return guidLetWindows, guidLetWindows, true
	case LegacyConsole:
		return guidConhost, guidConhost, true
	case WindowsTerminal:
		return guidWTConsole, guidWTTerminal, true
	case Unknown:
		return "", "", false
	default:
		return "", "", false
	}
}
