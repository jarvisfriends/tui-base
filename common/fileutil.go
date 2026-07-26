// Copyright (c) 2026 Jarvis Friends contributors
// SPDX-License-Identifier: MIT

package common

import (
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to path atomically: the bytes go to a temporary
// file in the same directory, which is then renamed over path. A crash or
// power loss mid-write leaves the previous file intact instead of a truncated
// one (B-2). The rename is atomic on POSIX filesystems and uses
// MoveFileEx(REPLACE_EXISTING) semantics on Windows.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	path = filepath.Clean(path)
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// Any failure below must not leave the temp file behind.
	defer func() {
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()
	// CreateTemp always uses 0o600; honor the caller's mode explicitly.
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	tmpName = "" // success — nothing to clean up
	return nil
}
