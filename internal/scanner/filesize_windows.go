//go:build windows

package scanner

import "os"

// fileSize returns the logical file size on Windows.
func fileSize(info os.FileInfo) int64 {
	return info.Size()
}
