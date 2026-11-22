package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	projectModule         = "github.com/zyato/personal"
	envRuntimeRoot        = "PERSONAL_ROOT"
	projectDataDirName    = ".local_file_rename"
	standaloneDataDirName = "local_file_rename"
	runtimeAuditLogName   = "audit.jsonl"
	runtimeLockName       = "run.lock"
)

// locateRuntimeDataDir resolves the directory that stores the process lock and
// the default audit log. A compiled binary must not require the Go source
// project to exist, and it must not scatter runtime state into an arbitrary
// working directory:
//
//  1. PERSONAL_ROOT, when set, uses <PERSONAL_ROOT>/.local_file_rename.
//  2. Inside this source project, the module root is detected automatically.
//  3. A standalone binary uses the host's user configuration directory.
func locateRuntimeDataDir() (string, error) {
	if root := strings.TrimSpace(os.Getenv(envRuntimeRoot)); root != "" {
		resolvedRoot, err := resolveExistingDirectory(root, envRuntimeRoot)
		if err != nil {
			return "", err
		}
		return runtimeDataDir(resolvedRoot), nil
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取当前目录失败: %w", err)
	}
	if projectRoot, ok := locateSourceProjectRoot(workingDirectory); ok {
		return runtimeDataDir(projectRoot), nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("无法确定独立数据目录；请设置 %s 或检查用户配置目录: %w", envRuntimeRoot, err)
	}
	if strings.TrimSpace(configDir) == "" {
		return "", fmt.Errorf("用户配置目录为空；请设置 %s", envRuntimeRoot)
	}
	return filepath.Join(configDir, standaloneDataDirName), nil
}

func locateSourceProjectRoot(start string) (string, bool) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		if isExpectedProjectRoot(current) {
			return current, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

func resolveExistingDirectory(path, label string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("解析%s失败: %w", label, err)
	}
	if absolute == string(filepath.Separator) {
		return "", fmt.Errorf("%s不能指向文件系统根目录", label)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("访问%s失败: %w", label, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s必须指向一个目录", label)
	}
	return filepath.Clean(absolute), nil
}

func isExpectedProjectRoot(directory string) bool {
	goModPath := filepath.Join(directory, "go.mod")
	info, err := os.Stat(goModPath)
	return err == nil && !info.IsDir() && isExpectedModule(goModPath)
}

func isExpectedModule(goModPath string) bool {
	raw, err := os.ReadFile(goModPath)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1] == projectModule
		}
	}
	return false
}

func runtimeDataDir(runtimeRoot string) string {
	return filepath.Join(runtimeRoot, projectDataDirName)
}

func runtimeAuditLogPath(dataDir string) string {
	return filepath.Join(dataDir, runtimeAuditLogName)
}

func runtimeLockPath(dataDir string) string {
	return filepath.Join(dataDir, runtimeLockName)
}

// ensureDataDir creates the dedicated runtime directory used by the lock and
// the default audit log. It intentionally does not migrate or touch files from
// older layouts that placed them directly beside the project root.
func ensureDataDir(dataDir string) error {
	info, err := os.Stat(dataDir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("数据目录 %s 已存在但不是目录", dataDir)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("检查数据目录失败: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}
	return nil
}

func sameFilePath(first, second string) bool {
	firstAbsolute, firstErr := filepath.Abs(first)
	secondAbsolute, secondErr := filepath.Abs(second)
	if firstErr == nil && secondErr == nil && filepath.Clean(firstAbsolute) == filepath.Clean(secondAbsolute) {
		return true
	}
	firstInfo, firstStatErr := os.Stat(first)
	secondInfo, secondStatErr := os.Stat(second)
	return firstStatErr == nil && secondStatErr == nil && os.SameFile(firstInfo, secondInfo)
}
