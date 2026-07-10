package status

import (
	"strings"
	"testing"

	"github.com/jarvisfriends/snap/keys"
)

// TestStatusSegments verifies the E-1 consumer hook: named segments render
// right-aligned in registration order, empty results are skipped per frame,
// and a nil fn removes the segment.
func TestStatusSegments(t *testing.T) {
	t.Parallel()

	b := New()
	b.SetKeys(keys.DefaultKeyMap())
	connection := "online"
	b.SetSegment("git", func() string { return "main*" })
	b.SetSegment("conn", func() string { return connection })
	b.SetSummaryProvider(func() string { return "heap 9MiB" })
	b.SetWidth(120)

	frame := stripANSI(b.View().Content)
	idxGit := strings.Index(frame, "main*")
	idxConn := strings.Index(frame, "online")
	idxSum := strings.Index(frame, "heap 9MiB")
	if idxGit < 0 || idxConn < 0 || idxSum < 0 {
		t.Fatalf("missing segment(s) in frame: git=%d conn=%d summary=%d\n%s",
			idxGit, idxConn, idxSum, frame)
	}
	if idxGit >= idxConn || idxConn >= idxSum {
		t.Fatalf("segments out of order: git=%d conn=%d summary=%d", idxGit, idxConn, idxSum)
	}

	// Empty result: skipped for the frame, no dangling separator.
	connection = ""
	b.SetWidth(120)
	frame = stripANSI(b.View().Content)
	if strings.Contains(frame, "online") {
		t.Fatal("empty segment still rendered")
	}
	if strings.Contains(frame, "•  •") {
		t.Fatalf("dangling separator after empty segment:\n%s", frame)
	}

	// nil fn removes the segment.
	b.SetSegment("git", nil)
	b.SetWidth(120)
	frame = stripANSI(b.View().Content)
	if strings.Contains(frame, "main*") {
		t.Fatal("removed segment still rendered")
	}
}
