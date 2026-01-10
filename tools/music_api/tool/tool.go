package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type MyColor string

// ANSI转义码
const (
	ColorReset   MyColor = "\033[0m"
	ColorBlack   MyColor = "\033[40m"
	ColorRed     MyColor = "\033[41m"
	ColorGreen   MyColor = "\033[42m"
	ColorYellow  MyColor = "\033[43m"
	ColorBlue    MyColor = "\033[44m"
	ColorMagenta MyColor = "\033[45m"
	ColorCyan    MyColor = "\033[46m"
	ColorWhite   MyColor = "\033[47m"
)

const (
	DownloadPath = "D:\\code\\src\\github.com\\zyato\\data\\vscode\\music\\"
	Sep          = "###"
)

func MyPrintln(str string, color MyColor) {
	fmt.Println(string(color) + str + string(ColorReset))
}

func LoadExistsSongs(folderPath string) (map[string]bool, error) {
	names, err := getAllFileNames(folderPath)
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(names)*3/2)
	for _, name := range names {
		m[name[:strings.Index(name, "-")]] = true
	}
	return m, nil
}

// getAllFileNames 返回指定文件夹下的所有文件名
func getAllFileNames(folderPath string) ([]string, error) {
	var fileNames []string
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 忽略文件夹本身
		if path != folderPath {
			fileNames = append(fileNames, info.Name())
		}
		return nil
	})
	return fileNames, err
}

// 根据names.txt音乐信息，转换成“歌名 歌手”的形式
func AdaptSongInfo(line []byte) []byte {
	s := string(line)
	s = strings.TrimSpace(s)
	return []byte(s + Sep + " ")
}
