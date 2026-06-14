//go:build !windows

package hashcache

import (
	"syscall"
)

func inodeFromStat(path string) uint64 {
	var stat syscall.Stat_t
	if err := syscall.Stat(path, &stat); err != nil {
		return 0
	}
	return stat.Ino
}
