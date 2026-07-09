package winterm

import "testing"

func TestDelegationGUIDRoundTrip(t *testing.T) {
	for _, d := range []Delegation{LetWindowsDecide, LegacyConsole, WindowsTerminal} {
		consoleGUID, terminalGUID, ok := guidsForDelegation(d)
		if !ok {
			t.Fatalf("%v: expected a defined GUID pair", d)
		}
		if got := delegationFromGUIDs(consoleGUID, terminalGUID); got != d {
			t.Errorf("%v: round trip produced %v", d, got)
		}
	}
}

func TestUnknownDelegation(t *testing.T) {
	if _, _, ok := guidsForDelegation(Unknown); ok {
		t.Error("Unknown must not map to a GUID pair")
	}
	// A mixed pair (e.g. after a partial write or a preview build) is Unknown,
	// never silently coerced to a known host.
	if got := delegationFromGUIDs(guidWTConsole, guidConhost); got != Unknown {
		t.Errorf("mixed pair = %v, want Unknown", got)
	}
	if got := delegationFromGUIDs(
		"{DEADBEEF-0000-0000-0000-000000000000}",
		guidWTTerminal,
	); got != Unknown {
		t.Errorf("unrecognized console GUID = %v, want Unknown", got)
	}
}

func TestDelegationString(t *testing.T) {
	cases := map[Delegation]string{
		LetWindowsDecide: "let Windows decide",
		LegacyConsole:    "legacy console (conhost)",
		WindowsTerminal:  "Windows Terminal",
		Unknown:          "unknown",
		Delegation(99):   "unknown",
	}
	for d, want := range cases {
		if got := d.String(); got != want {
			t.Errorf("Delegation(%d).String() = %q, want %q", d, got, want)
		}
	}
}
