package krc_to_lrc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Run(path string) (string, error) {
	// 参数校验
	stat, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat: %w", err)
	}
	if !stat.Mode().IsRegular() {
		return "", fmt.Errorf("not regular: %w", err)
	}

	// 解析 krc 文件
	krc, err := parseKRCFile(path)
	if err != nil {
		return "", fmt.Errorf("parseKRCFile: %w", err)
	}
	if len(krc) == 0 {
		return "", fmt.Errorf("krc file is empty")
	}

	// 格式转换成 lrc

	// 移除 krc 逐字时间
	krcWithoutWordTime := wordTimeRe.ReplaceAllString(krc, "")
	// 格式化 krc 行时间为 lrc 时间
	lrc := formatToLRCTime(krcWithoutWordTime)

	return lrc, nil
}

/*
*
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o krc2lrc
*/
func CMD() {
	path := "1.krc"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	lrc, err := Run(path)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	dir, name := getPathInfo(path)
	// 保存 lrc 文件
	saveFile(dir, name+".lrc", lrc)
}

func getPathInfo(path string) (string, string) {
	if strings.HasSuffix(strings.ToLower(path), ".krc") {
		path = path[:len(path)-4]
	}
	dir, file := filepath.Split(path)
	return dir, file
}

func saveFile(dir, name, content string) {
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
	if err != nil {
		fmt.Println("save file fail: ", filepath.Join(dir, name), err)
	}
}
