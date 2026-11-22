package main

import (
	"fmt"
	"os"
	"time"
)

// acquireProcessLock prevents two local rename runs from planning and applying
// against the same directory concurrently. A stale lock after SIGKILL must be
// removed manually after confirming no instance is running.
func acquireProcessLock(lockPath string) (func(), error) {
	if lockPath == "" {
		return nil, fmt.Errorf("运行锁路径不能为空")
	}
	file, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("检测到另一个实例或遗留锁文件 %s；确认无实例运行后才能清理该锁文件", lockPath)
		}
		return nil, fmt.Errorf("创建运行锁失败: %w", err)
	}
	_, _ = fmt.Fprintf(file, "pid=%d\ntime=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	if err := file.Close(); err != nil {
		_ = os.Remove(lockPath)
		return nil, fmt.Errorf("写入运行锁失败: %w", err)
	}
	return func() { _ = os.Remove(lockPath) }, nil
}
