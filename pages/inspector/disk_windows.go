//go:build windows

package inspector

import "golang.org/x/sys/windows"

// listDriveStats enumerates every logical drive on Windows and returns space
// information for each one that responds to GetDiskFreeSpaceEx.
func listDriveStats() []diskStat {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return []diskStat{{Path: "?", Error: err.Error()}}
	}

	var stats []diskStat
	for i := range 26 {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		letter := string(rune('A'+i)) + ":\\"

		var freeToCaller, totalBytes, totalFree uint64
		ptr, err := windows.UTF16PtrFromString(letter)
		if err != nil {
			stats = append(stats, diskStat{Path: letter, Error: err.Error()})
			continue
		}
		if err := windows.GetDiskFreeSpaceEx(
			ptr,
			&freeToCaller,
			&totalBytes,
			&totalFree,
		); err != nil {
			stats = append(stats, diskStat{Path: letter, Error: err.Error()})
			continue
		}
		stats = append(stats, diskStat{
			Path:  letter,
			Total: totalBytes,
			Free:  freeToCaller, // respects per-user quotas
			Used:  totalBytes - freeToCaller,
		})
	}
	return stats
}
