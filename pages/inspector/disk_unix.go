//go:build !windows

package inspector

import (
	"bufio"
	"log"
	"os"
	"strings"
	"syscall"
)

// listDriveStats enumerates mount points from /proc/mounts (Linux) or falls
// back to "/" on other Unix systems, then returns space info via Statfs.
func listDriveStats() []diskStat {
	mounts := collectMountPoints()
	var stats []diskStat
	for _, mp := range mounts {
		if strings.Contains(mp, "snap") || strings.Contains(mp, "/export") {
			continue // skip snap mounts that cause Statfs to fail
		}
		var st syscall.Statfs_t
		if err := syscall.Statfs(mp, &st); err != nil {
			continue // skip inaccessible or virtual fs
		}
		if st.Blocks == 0 {
			continue // zero-size: tmpfs, cgroup, etc.
		}
		bsize := uint64(max(0, st.Bsize))
		total := st.Blocks * bsize
		free := st.Bavail * bsize
		stats = append(stats, diskStat{
			Path:  mp,
			Total: total,
			Free:  free,
			Used:  total - free,
		})
	}
	if len(stats) == 0 {
		return []diskStat{{Path: "/", Error: "no physical mounts found"}}
	}
	return stats
}

// collectMountPoints reads /proc/mounts on Linux; falls back to ["/"] elsewhere.
func collectMountPoints() []string {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return []string{"/"}
	}
	defer func() {
		e := f.Close()
		if e != nil {
			// handle error if needed
			log.Default().Printf("warning: failed to close /proc/mounts: %v", e)
		}
	}()

	// Filesystem types that carry no real disk data.
	skipTypes := map[string]bool{
		"proc": true, "sysfs": true, "devtmpfs": true, "devpts": true,
		"tmpfs": true, "cgroup": true, "cgroup2": true, "debugfs": true,
		"tracefs": true, "securityfs": true, "pstore": true, "bpf": true,
		"fusectl": true, "hugetlbfs": true, "mqueue": true, "configfs": true,
		"efivarfs": true, "none": true, "overlay": true, "aufs": true,
		"rpc_pipefs": true, "nfsd": true,
	}

	seen := map[string]bool{}
	var mounts []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		mp, fsType := fields[1], fields[2]
		if skipTypes[fsType] || seen[mp] {
			continue
		}
		seen[mp] = true
		mounts = append(mounts, mp)
	}
	if len(mounts) == 0 {
		return []string{"/"}
	}
	return mounts
}
