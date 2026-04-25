//go:build !windows
// +build !windows

package main

// resolveUNC is a no-op on non-Windows platforms.
// Drive letter to UNC resolution is only needed on Windows.
func resolveUNC(path string) string {
	return path
}
