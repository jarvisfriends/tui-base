package gate

import "testing"

func TestHas(t *testing.T) {
	g := NewGateRegistry()
	if g.Has("absent") {
		t.Error("Has reported an unregistered gate")
	}
	g.Register(FeatureGate{Name: "present", Default: false})
	if !g.Has("present") {
		t.Error("Has missed a registered gate")
	}
	// Value's absent-means-enabled default must not leak into Has.
	if !g.Value("absent") {
		t.Error("Value contract changed: absent gate should read enabled")
	}
	if g.Has("absent") {
		t.Error("Has reported an absent gate after other registrations")
	}
}
