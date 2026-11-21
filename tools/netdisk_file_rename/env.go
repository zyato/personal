package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// loadDotEnv loads simple KEY=VALUE pairs without executing shell expressions.
// Existing process environment variables take precedence over the file.
func loadDotEnv(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("打开配置文件失败: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("配置文件第 %d 行缺少等号", lineNumber)
		}
		key = strings.TrimSpace(key)
		if !validEnvKey(key) {
			return fmt.Errorf("配置文件第 %d 行包含无效变量名", lineNumber)
		}
		value, err := parseEnvValue(strings.TrimSpace(rawValue))
		if err != nil {
			return fmt.Errorf("配置文件第 %d 行: %w", lineNumber, err)
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("设置配置项 %s 失败: %w", key, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	return nil
}

func validEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for index, char := range key {
		if char == '_' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || index > 0 && char >= '0' && char <= '9' {
			continue
		}
		return false
	}
	return true
}

func parseEnvValue(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if raw[0] == '\'' {
		if len(raw) < 2 || raw[len(raw)-1] != '\'' {
			return "", fmt.Errorf("单引号未闭合")
		}
		return raw[1 : len(raw)-1], nil
	}
	if raw[0] == '"' {
		value, err := strconv.Unquote(raw)
		if err != nil {
			return "", fmt.Errorf("双引号格式错误")
		}
		return value, nil
	}
	// An unquoted # starts a comment only when preceded by whitespace.
	for index, char := range raw {
		if char == '#' && index > 0 && (raw[index-1] == ' ' || raw[index-1] == '\t') {
			raw = raw[:index]
			break
		}
	}
	return strings.TrimSpace(raw), nil
}
