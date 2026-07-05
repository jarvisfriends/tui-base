package notifications_test

import (
	"fmt"
	"time"

	"github.com/jarvisfriends/tui-base/notifications"
)

// ExampleManager_Add shows posting a notification to the shared manager; the
// router renders it as a toast and lists it in the history panel until the
// TTL expires or the user dismisses it. Ordering is newest-first with the
// monotonic ID as tie-break, so it is identical on every platform even when
// entries share a coarse-clock timestamp.
func ExampleManager_Add() {
	nm := notifications.NewManager()
	nm.Add("Deploy finished", notifications.SeverityInfo, 5*time.Second)
	fmt.Println(len(nm.Active()))
	// Output: 1
}
