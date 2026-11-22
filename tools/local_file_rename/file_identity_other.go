//go:build !unix

package main

import (
	"fmt"
	"os"
)

func fileIdentity(_ os.FileInfo) (string, error) {
	return "", fmt.Errorf("当前平台不支持获取稳定文件标识，已停止以避免误重命名")
}
