package status

import (
	"testing"

	"github.com/jarvisfriends/snap/keys"
)

func BenchmarkSetWidth(b *testing.B) {
	s := New()
	s.SetKeys(keys.DefaultKeyMap())
	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		s.SetWidth(80 + (i % 20))
	}
}

func BenchmarkHeight(b *testing.B) {
	s := New()
	s.SetKeys(keys.DefaultKeyMap())
	s.SetWidth(80)
	b.ReportAllocs()

	for b.Loop() {
		_ = s.Height()
	}
}
