//go:build windows
// +build windows

package main

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

var (
	mpr                    = syscall.NewLazyDLL("mpr.dll")
	procWNetGetConnectionW = mpr.NewProc("WNetGetConnectionW")
)

// resolveUNC converts a mapped drive letter path (e.g. H:\ptxrid\in) to its
// UNC equivalent (e.g. \\server\share\ptxrid\in). This is critical on Windows 7
// where UAC elevation creates a new logon session that may not have the
// drive mappings of the original user session.
//
// If the path is not a mapped drive or resolution fails, the original path
// is returned unchanged.
func resolveUNC(path string) string {
	if len(path) < 2 || path[1] != ':' {
		return path // Not a drive-letter path
	}

	driveLetter := strings.ToUpper(path[:2]) // e.g. "H:"
	remainder := path[2:]                     // e.g. "\ptxrid\in"

	// Allocate a buffer for the UNC path
	bufSize := uint32(1024)
	buf := make([]uint16, bufSize)

	localName, err := syscall.UTF16PtrFromString(driveLetter)
	if err != nil {
		return path
	}

	ret, _, _ := procWNetGetConnectionW.Call(
		uintptr(unsafe.Pointer(localName)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bufSize)),
	)

	if ret != 0 {
		// WNetGetConnection failed — drive is likely a local disk, not mapped.
		// Return the original path unchanged.
		return path
	}

	uncRoot := syscall.UTF16ToString(buf)
	resolved := uncRoot + remainder

	fmt.Printf(">> Lecteur %s resolu vers UNC : %s\n", driveLetter, resolved)
	return resolved
}
