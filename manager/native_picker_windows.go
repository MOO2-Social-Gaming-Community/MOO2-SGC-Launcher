//go:build windows

package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// These are fixed Win32 common dialogs. No PowerShell, shell interpolation,
// executable supplied by a web request, or file upload is involved.
type browseInfo struct {
	Owner       uintptr
	Root        uintptr
	DisplayName *uint16
	Title       *uint16
	Flags       uint32
	Callback    uintptr
	Param       uintptr
	Image       int32
}
type openFileName struct {
	Size             uint32
	Owner            uintptr
	Instance         uintptr
	Filter           *uint16
	CustomFilter     *uint16
	MaxCustomFilter  uint32
	FilterIndex      uint32
	File             *uint16
	MaxFile          uint32
	FileTitle        *uint16
	MaxFileTitle     uint32
	InitialDir       *uint16
	Title            *uint16
	Flags            uint32
	FileOffset       uint16
	FileExtension    uint16
	DefaultExtension *uint16
	CustomData       uintptr
	Hook             uintptr
	TemplateName     *uint16
	Reserved         uintptr
	Reserved2        uint32
	FlagsEx          uint32
}

func nativePick(kind string) (string, error) {
	folder, title, filter, err := pickerType(kind)
	if err != nil {
		return "", err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole := syscall.NewLazyDLL("ole32.dll")
	hr, _, _ := ole.NewProc("CoInitializeEx").Call(0, 2)
	if int32(hr) < 0 {
		return "", fmt.Errorf("Windows file chooser initialization failed (0x%x)", hr)
	}
	defer ole.NewProc("CoUninitialize").Call()
	t, _ := syscall.UTF16PtrFromString(title)
	buf := make([]uint16, 32768)
	if folder {
		shell := syscall.NewLazyDLL("shell32.dll")
		info := browseInfo{DisplayName: &buf[0], Title: t, Flags: 0x0041} // filesystem folders; modern dialog
		pidl, _, _ := shell.NewProc("SHBrowseForFolderW").Call(uintptr(unsafe.Pointer(&info)))
		if pidl == 0 {
			return "", nil
		}
		defer ole.NewProc("CoTaskMemFree").Call(pidl)
		ok, _, _ := shell.NewProc("SHGetPathFromIDListEx").Call(pidl, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
		if ok == 0 {
			return "", fmt.Errorf("choose a local filesystem folder")
		}
	} else {
		// UTF16FromString deliberately rejects embedded NUL; filters need pairs.
		f := utf16.Encode([]rune(filter))
		info := openFileName{File: &buf[0], MaxFile: uint32(len(buf)), Filter: &f[0], FilterIndex: 1, Title: t, Flags: 0x1000 | 0x0800 | 0x0008 | 0x00080000 | 0x02000000}
		info.Size = uint32(unsafe.Sizeof(info))
		dialog := syscall.NewLazyDLL("comdlg32.dll")
		ok, _, _ := dialog.NewProc("GetOpenFileNameW").Call(uintptr(unsafe.Pointer(&info)))
		if ok == 0 {
			code, _, _ := dialog.NewProc("CommDlgExtendedError").Call()
			if code != 0 {
				return "", fmt.Errorf("Windows file chooser failed (0x%x)", code)
			}
			return "", nil
		}
	}
	return syscall.UTF16ToString(buf), nil
}
