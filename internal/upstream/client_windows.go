//go:build windows

package upstream

import (
	"fmt"
	"syscall"
	"unsafe"
)

// getOSInfo returns OS name and version for Windows
// Format: "Windows {major}.{minor}.{build}" (e.g., "Windows 10.0.22000")
func getOSInfo() string {
	// Use RtlGetVersion from ntdll.dll for accurate Windows version
	// (GetVersionExW lies about version on Windows 8.1+)
	ntdll := syscall.NewLazyDLL("ntdll.dll")
	proc := ntdll.NewProc("RtlGetVersion")

	type osVersionInfoEx struct {
		OSVersionInfoSize uint32
		MajorVersion      uint32
		MinorVersion      uint32
		BuildNumber       uint32
		PlatformId        uint32
		CSDVersion        [128]uint16 // We don't need this but it's part of the struct
	}

	var info osVersionInfoEx
	info.OSVersionInfoSize = uint32(unsafe.Sizeof(info))

	ret, _, _ := proc.Call(uintptr(unsafe.Pointer(&info)))
	if ret != 0 {
		// RtlGetVersion returns non-zero on failure
		return "Windows"
	}

	return fmt.Sprintf("Windows %d.%d.%d",
		info.MajorVersion, info.MinorVersion, info.BuildNumber)
}
