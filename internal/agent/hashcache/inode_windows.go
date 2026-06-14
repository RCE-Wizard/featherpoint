//go:build windows

package hashcache

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

func inodeFromStat(path string) uint64 {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)

	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return 0
	}
	_ = unsafe.Sizeof(info)
	return uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow)
}
