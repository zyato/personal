package krc_to_lrc

import (
	"fmt"
	"strconv"
	"strings"
)

// 将 KRC 的行时间 [start,len] 替换为 LRC 时间 [mm:ss.xx]
func formatToLRCTime(data string) string {
	var (
		lastTime int
		result   strings.Builder
	)
	result.Grow(len(data))

	lines := strings.Split(data, "\n")
	for _, line := range lines {
		matches := lineTimeRe.FindStringSubmatchIndex(line)
		// 本行非歌词，直接写入
		if matches == nil || len(matches) != 6 {
			result.WriteString(line)
			result.WriteByte('\n')
			continue
		}
		// matches: [fullStart, fullEnd, g1Start, g1End, g2Start, g2End]
		// 当前行歌词开始时间（绝对时间）
		currentTime, err := strconv.Atoi(line[matches[2]:matches[3]])
		if err != nil {
			result.WriteString(line)
			result.WriteByte('\n')
			continue
		}
		// 当前行歌词持续时间（毫秒）
		duration, err := strconv.Atoi(line[matches[4]:matches[5]])
		if err != nil {
			result.WriteString(line)
			result.WriteByte('\n')
			continue
		}
		// 整个 [x,y] 的字符串长度
		timeStrLen := matches[1]

		if lastTime < currentTime-emptyLineIntervalTime {
			result.WriteString(makeLRCTime(lastTime))
			result.WriteByte('\n')
		}
		result.WriteString(makeLRCTime(currentTime))
		result.WriteString(line[timeStrLen:])
		result.WriteByte('\n')

		lastTime = currentTime + duration
	}

	return result.String()
}

// 将当前进行时间（毫秒）转换为 [mm:ss.xx] 的格式
func makeLRCTime(time int) string {
	minutes := time / 60_000
	seconds := (time % 60_000) / 1000
	mill10 := (time % 1000) / 10

	return fmt.Sprintf("[%02d:%02d.%02d]", minutes, seconds, mill10)
}
