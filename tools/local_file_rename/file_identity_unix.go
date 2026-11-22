//go:build unix

package main

import (
	"fmt"
	"os"
	"syscall"
)

// fileIdentity returns a stable dev/inode identity for a regular file. It is
// the local equivalent of the netdisk fs_id and lets a rebuilt plan detect a
// file that was replaced between preview and execution.
func fileIdentity(info os.FileInfo) (string, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("系统未返回文件标识信息")
	}
	return fmt.Sprintf("dev=%d;ino=%d", stat.Dev, stat.Ino), nil
}
