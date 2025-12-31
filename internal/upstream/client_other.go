//go:build !darwin && !linux && !windows

package upstream

import "runtime"

// getOSInfo returns OS name for unsupported platforms
// Falls back to just the OS name from runtime.GOOS
func getOSInfo() string {
	return runtime.GOOS
}
