package inspector

import "testing"

func FuzzLogMessage(f *testing.F) {
	f.Add("seed")
	f.Fuzz(func(t *testing.T, s string) {
		m := New()
		// Pass arbitrary string messages to ensure logging is robust.
		_, _ = m.Update(s)
	})
}
