//go:build !windows

package winterm

import (
	"errors"
	"fmt"
)

// errNotWindows is returned by Detect and Set off Windows; it matches
// errors.Is(err, errors.ErrUnsupported).
var errNotWindows = fmt.Errorf(
	"winterm: default-terminal delegation is windows-only: %w", errors.ErrUnsupported,
)

// Detect is Windows-only; elsewhere it reports Unknown with an error matching
// [errors.ErrUnsupported].
func Detect() (Delegation, error) { return Unknown, errNotWindows }

// Set is Windows-only; elsewhere it returns an error matching
// [errors.ErrUnsupported].
func Set(Delegation) error { return errNotWindows }
