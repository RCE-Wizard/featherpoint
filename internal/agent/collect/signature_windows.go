//go:build windows

package collect

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

// signatureOf checks Windows Authenticode signature via WinVerifyTrust.
func signatureOf(path string) (bool, string) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, ""
	}

	// WINTRUST_FILE_INFO
	fileInfo := struct {
		cbStruct  uint32
		pcwszFilePath *uint16
		hFile     uintptr
		pgKnownSubject *windows.GUID
	}{
		cbStruct:      52,
		pcwszFilePath: p,
	}

	actionGUID := windows.GUID{
		Data1: 0xaac56b, Data2: 0xcd44, Data3: 0x11d0,
		Data4: [8]byte{0x8c, 0xc2, 0x00, 0xc0, 0x4f, 0xc2, 0x95, 0xee},
	}

	// WINTRUST_DATA
	data := struct {
		cbStruct            uint32
		pPolicyCallbackData uintptr
		pSIPClientData      uintptr
		dwUIChoice          uint32
		fdwRevocationChecks uint32
		dwUnionChoice       uint32
		pFile               uintptr
		dwStateAction       uint32
		hWVTStateData       uintptr
		pwszURLReference    uintptr
		dwProvFlags         uint32
		dwUIContext          uint32
	}{
		cbStruct:      uint32(unsafe.Sizeof(data)),
		dwUIChoice:    2,  // WTD_UI_NONE
		dwUnionChoice: 1,  // WTD_CHOICE_FILE
		pFile:         uintptr(unsafe.Pointer(&fileInfo)),
	}

	wintrust := windows.NewLazySystemDLL("wintrust.dll")
	proc := wintrust.NewProc("WinVerifyTrust")
	ret, _, _ := proc.Call(uintptr(windows.InvalidHandle), uintptr(unsafe.Pointer(&actionGUID)), uintptr(unsafe.Pointer(&data)))
	return ret == 0, ""
}
