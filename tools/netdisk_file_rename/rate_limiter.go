package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type rateLimitState struct {
	Requests []time.Time `json:"requests"`
}

// rateLimiter persists request timestamps so repeated invocations cannot bypass
// the local hourly budget. The CLI also holds a lock to prevent parallel runs.
type rateLimiter struct {
	mu          sync.Mutex
	minInterval time.Duration
	maxPerHour  int
	statePath   string
	requests    []time.Time
	now         func() time.Time
}

func newRateLimiter(minInterval time.Duration, maxPerHour int, statePath string) (*rateLimiter, error) {
	limiter := &rateLimiter{
		minInterval: minInterval,
		maxPerHour:  maxPerHour,
		statePath:   statePath,
		now:         time.Now,
	}
	if statePath == "" {
		return limiter, nil
	}
	raw, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return limiter, nil
		}
		return nil, fmt.Errorf("读取限速状态失败: %w", err)
	}
	var state rateLimitState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("限速状态文件损坏，为避免绕过限频已停止: %w", err)
	}
	now := limiter.now().UTC()
	for _, requestTime := range state.Requests {
		if requestTime.After(now.Add(5 * time.Minute)) {
			return nil, fmt.Errorf("限速状态包含异常的未来时间，请检查系统时钟")
		}
		if requestTime.After(now.Add(-time.Hour)) {
			limiter.requests = append(limiter.requests, requestTime)
		}
	}
	return limiter, nil
}

func (limiter *rateLimiter) wait(ctx context.Context) error {
	for {
		limiter.mu.Lock()
		now := limiter.now().UTC()
		limiter.pruneLocked(now)
		waitFor := limiter.waitDurationLocked(now)
		if waitFor <= 0 {
			limiter.requests = append(limiter.requests, now)
			if err := limiter.saveLocked(); err != nil {
				limiter.requests = limiter.requests[:len(limiter.requests)-1]
				limiter.mu.Unlock()
				return err
			}
			limiter.mu.Unlock()
			return nil
		}
		limiter.mu.Unlock()

		timer := time.NewTimer(waitFor)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (limiter *rateLimiter) waitDurationLocked(now time.Time) time.Duration {
	var waitFor time.Duration
	if limiter.maxPerHour > 0 && len(limiter.requests) >= limiter.maxPerHour {
		waitFor = limiter.requests[0].Add(time.Hour).Sub(now)
	}
	if limiter.minInterval > 0 && len(limiter.requests) > 0 {
		intervalWait := limiter.requests[len(limiter.requests)-1].Add(limiter.minInterval).Sub(now)
		if intervalWait > waitFor {
			waitFor = intervalWait
		}
	}
	return waitFor
}

func (limiter *rateLimiter) pruneLocked(now time.Time) {
	first := 0
	for first < len(limiter.requests) && !limiter.requests[first].After(now.Add(-time.Hour)) {
		first++
	}
	if first > 0 {
		limiter.requests = append(limiter.requests[:0], limiter.requests[first:]...)
	}
}

func (limiter *rateLimiter) saveLocked() error {
	if limiter.statePath == "" {
		return nil
	}
	state := rateLimitState{Requests: limiter.requests}
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("编码限速状态失败: %w", err)
	}
	directory := filepath.Dir(limiter.statePath)
	temporary, err := os.CreateTemp(directory, ".netdisk-rate-*")
	if err != nil {
		return fmt.Errorf("创建限速状态临时文件失败: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("设置限速状态权限失败: %w", err)
	}
	if _, err := temporary.Write(raw); err != nil {
		temporary.Close()
		return fmt.Errorf("写入限速状态失败: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("同步限速状态失败: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("关闭限速状态失败: %w", err)
	}
	if err := os.Rename(temporaryName, limiter.statePath); err != nil {
		return fmt.Errorf("保存限速状态失败: %w", err)
	}
	return nil
}

func acquireProcessLock(statePath string) (func(), error) {
	if statePath == "" {
		return nil, fmt.Errorf("限速状态文件路径不能为空")
	}
	lockPath := statePath + ".lock"
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
