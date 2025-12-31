//go:build linux

package upstream

import (
	"syscall"
)

// getOSInfo returns OS name and version for Linux
// Format: "Linux {kernel_version}" (e.g., "Linux 6.5.0")
func getOSInfo() string {
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err != nil {
		return "Linux"
	}
	release := int8ArrayToString(uname.Release[:])
	if release == "" {
		return "Linux"
	}
	return "Linux " + release
}

// int8ArrayToString converts a null-terminated int8 array to a string
func int8ArrayToString(arr []int8) string {
	var b []byte
	for _, v := range arr {
		if v == 0 {
			break
		}
		b = append(b, byte(v))
	}
	return string(b)
}
