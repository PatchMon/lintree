//go:build !windows

package scanner

import (
	"os"
	"syscall"
)

// fileSize returns the disk usage for a file. On Unix systems this uses
// stat.Blocks * 512 to get actual disk usage rather than logical file size.
func fileSize(info os.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Blocks * 512
	}
	return info.Size()
}
