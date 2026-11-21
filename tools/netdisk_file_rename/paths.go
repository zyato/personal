package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const projectModule = "github.com/zyato/personal"

// locateProjectRoot anchors .env, the rate state and the process lock to one
// project, even when the command is invoked from a nested working directory.
func locateProjectRoot() (string, error) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取当前目录失败: %w", err)
	}
	return locateProjectRootFrom(workingDirectory)
}

func locateProjectRootFrom(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("解析项目目录失败: %w", err)
	}
	for {
		goModPath := filepath.Join(current, "go.mod")
		info, statErr := os.Stat(goModPath)
		if statErr == nil && !info.IsDir() && isExpectedModule(goModPath) {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("未找到模块 %s 的项目根目录；请在本项目目录内运行", projectModule)
		}
		current = parent
	}
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
