package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const maxLocalEntries = 100000

// listLocalEntries reads direct children without recursion. Directories and
// non-regular entries (symlinks, sockets, devices, and so on) remain visible
// to conflict detection, but only regular files can be planned for renaming.
func listLocalEntries(directory string) ([]FileEntry, error) {
	directoryEntries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("读取本地目录失败: %w", err)
	}
	entries := make([]FileEntry, 0, len(directoryEntries))
	for _, directoryEntry := range directoryEntries {
		name := directoryEntry.Name()
		path := filepath.Join(directory, name)
		info, err := directoryEntry.Info()
		if err != nil {
			return nil, fmt.Errorf("读取条目 %s 的信息失败: %w", path, err)
		}
		entry := FileEntry{
			Path:  path,
			Name:  name,
			IsDir: info.IsDir(),
		}
		if !entry.IsDir && info.Mode().IsRegular() {
			fileID, idErr := fileIdentity(info)
			if idErr != nil {
				return nil, fmt.Errorf("获取文件 %s 的稳定标识失败: %w", path, idErr)
			}
			entry.FileID = fileID
		} else if !entry.IsDir {
			entry.Unsupported = true
		}
		entries = append(entries, entry)
		if len(entries) > maxLocalEntries {
			return nil, fmt.Errorf("目录条目超过 %d，为避免误操作已停止", maxLocalEntries)
		}
	}
	return entries, nil
}

// normalizeLocalDirectory resolves the requested directory to an existing,
// canonical directory path. The filesystem root is rejected on purpose.
func normalizeLocalDirectory(directory string) (string, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return "", fmt.Errorf("必须通过 --dir 指定本地目录")
	}
	if !utf8.ValidString(directory) || strings.ContainsRune(directory, '\x00') {
		return "", fmt.Errorf("目录包含无效字符")
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("解析本地目录失败: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(filepath.Clean(absolute))
	if err != nil {
		return "", fmt.Errorf("解析本地目录符号链接失败: %w", err)
	}
	if canonical == string(filepath.Separator) {
		return "", fmt.Errorf("不能指定文件系统根目录")
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", fmt.Errorf("访问本地目录失败: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("--dir 必须指向一个本地目录")
	}
	return canonical, nil
}

func targetPath(directory, newName string) string {
	return filepath.Join(directory, newName)
}

// verifyPlanSource ensures the path still refers to the exact regular file
// captured when the current plan was built.
func verifyPlanSource(plan RenamePlan) error {
	info, err := os.Lstat(plan.SourcePath)
	if err != nil {
		return fmt.Errorf("源文件已不存在或无法访问: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("源路径已不再是普通文件，已停止")
	}
	fileID, err := fileIdentity(info)
	if err != nil {
		return fmt.Errorf("获取源文件稳定标识失败: %w", err)
	}
	if fileID != plan.FileID {
		return fmt.Errorf("源文件标识与计划不一致，可能已被替换，已停止")
	}
	return nil
}

// ensureTargetAbsent is checked immediately before os.Rename. Together with the
// process lock and the rebuilt plan it prevents overwriting reviewed files;
// concurrent external changes to the directory are outside this tool's control.
func ensureTargetAbsent(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("目标文件已存在，已停止")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查目标文件失败: %w", err)
	}
	return nil
}

func renameLocalFile(plan RenamePlan, directory string) error {
	if err := verifyPlanSource(plan); err != nil {
		return err
	}
	destination := targetPath(directory, plan.NewName)
	if err := ensureTargetAbsent(destination); err != nil {
		return err
	}
	if err := os.Rename(plan.SourcePath, destination); err != nil {
		return fmt.Errorf("执行本地重命名失败: %w", err)
	}
	return nil
}
