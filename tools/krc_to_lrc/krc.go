package krc_to_lrc

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"regexp"
)

const (
	emptyLineIntervalTime = 500 // 歌词间隔时间超过500毫秒，新增一个空行
)

var (
	// KRC 解码后的歌词单字时间标记，如 <123,456,789>
	wordTimeRe = regexp.MustCompile(`<\d+,\d+,\d+>`)

	// KRC 解码后的歌词行时间 [start,len]
	lineTimeRe = regexp.MustCompile(`^\[(\d+),(\d+)\]`)
)

func parseKRCFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	// 前 3 个字节不是 "krc"
	if len(raw) < 3 || string(raw[:3]) != "krc" {
		return "", fmt.Errorf("file %v is not krc file", path)
	}

	parsed, err := decodeKRC(raw)
	if err != nil {
		return "", fmt.Errorf("decode krc file: %w", err)
	}

	return parsed, nil
}

// 解码 KRC（异或 + zlib）
func decodeKRC(data []byte) (string, error) {
	encodeKey := []byte{64, 71, 97, 119, 94, 50, 116, 71, 81, 54, 49, 45, 206, 210, 110, 105}
	buffer := make([]byte, 0, len(data)-4)
	for i := 4; i < len(data); i++ {
		buffer = append(buffer, data[i]^encodeKey[(i-4)%16])
	}

	var krcRaw bytes.Buffer
	r, err := zlib.NewReader(bytes.NewReader(buffer))
	if err != nil {
		return "", fmt.Errorf("zlib.NewReader error: %w", err)
	}
	defer r.Close()
	_, err = io.Copy(&krcRaw, r)
	if err != nil {
		return "", fmt.Errorf("io.Copy error: %w", err)
	}

	return krcRaw.String(), nil
}
