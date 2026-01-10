package qq

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"yato/code/api/tool"
)

const musicURL = `https://api.xingzhige.com/API/QQmusicVIP`

type MusicResp struct {
	Data MusicList `json:"data"`
}

type MusicData struct {
	SongName string `json:"songname"`
	Singer   string `json:"name"`
	Album    string `json:"album"`
	Mid      string `json:"mid"`
	Src      string `json:"src"`
}

type DownloadResp struct {
	Data MusicData `json:"data"`
}

type MusicList []MusicData

func (m MusicList) String() string {
	res := ""
	for i, v := range m {
		if i == 0 {
			res += fmt.Sprintf("%s%d: 歌名:%s 歌手:%s 专辑:%s%s\n", tool.ColorMagenta, i+1, v.SongName, v.Singer, v.Album, tool.ColorReset)
			continue
		}
		res += fmt.Sprintf("%d: 歌名:%s 歌手:%s 专辑:%s\n", i+1, v.SongName, v.Singer, v.Album)
	}
	return res
}

func Music(songListFile, downloadFailedFile *os.File, existSongs map[string]bool) {
	exportDownloadFail := func(str []byte) {
		downloadFailedFile.Write(str)
		downloadFailedFile.Write([]byte("\n"))
	}
	songListReader := bufio.NewReader(songListFile)
	for {
		line, _, err := songListReader.ReadLine()
		if err == io.EOF {
			break
		}
		ss := strings.Split(string(tool.AdaptSongInfo(line)), tool.Sep)
		if len(ss) != 2 {
			fmt.Println("数量不对: ", ss)
			exportDownloadFail(line)
			continue
		}
		if existSongs[ss[0]] {
			fmt.Printf("【已存在】歌名: %s, 歌手: %s\n", ss[0], ss[1])
			continue
		}
		tool.MyPrintln(fmt.Sprintf("歌名: %s, 歌手: %s", ss[0], ss[1]), tool.ColorMagenta)
		dataList, err := getMusicInfo(ss[0], ss[1])
		if err != nil {
			fmt.Println("获取音乐信息失败：", err)
			exportDownloadFail(line)
			continue
		}
		fmt.Printf("%v\n输入下载哪一个音乐源: ", dataList)
		var item int
		_, err = fmt.Scanf("%d\n", &item)
		if err != nil {
			item = 1
		}
		if item == 0 {
			exportDownloadFail(line)
			continue
		}
		s, err := getDownloadInfo(dataList[item-1].Mid)
		if err != nil {
			fmt.Println("音乐链接获取错误: ", err)
			exportDownloadFail(line)
			continue
		}
		fmt.Printf("开始下载第 %d 个资源: 歌名: %s, 歌手：%s\n", item, dataList[item-1].SongName, dataList[item-1].Singer)
		fileName, err := download(dataList[item-1].SongName, dataList[item-1].Singer, s)
		if err != nil {
			fmt.Println("下载失败: ", err)
			exportDownloadFail(line)
			continue
		}
		fmt.Printf("%s 下载成功\n\n\n", fileName)
	}
}

func download(name, singer, src string) (string, error) {
	u, err := url.Parse(src)
	if err != nil {
		return "", err
	}
	fileName := fmt.Sprintf("%s-%s%s", name, singer, u.Path[strings.LastIndex(u.Path, "."):])
	fileName = strings.ReplaceAll(fileName, "/", "、")
	r, err := http.Get(src)
	if err != nil {
		return "", fmt.Errorf(fileName + ":" + err.Error())
	}
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	if len(data) < 1024 {
		return "", fmt.Errorf(fmt.Sprintf("fileName: %s, 文件尺寸不对：%d", fileName, (data)))
	}
	os.WriteFile(tool.DownloadPath+fileName, data, 0666)
	return fileName, err
}

func getMusicInfo(name, singer string) (MusicList, error) {
	name = strings.TrimSpace(name)
	singer = strings.TrimSpace(singer)
	u := musicURL + fmt.Sprintf("?name=%s&br=11&type=json&max=10", name+singer)
	r, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	resp := &MusicResp{}
	err = json.Unmarshal(data, resp)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func getDownloadInfo(mid string) (string, error) {
	u := musicURL + fmt.Sprintf("?mid=%s&br=14", mid)
	r, err := http.Get(u)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	v := &DownloadResp{}
	err = json.Unmarshal(data, v)
	if err != nil {
		return "", err
	}
	return v.Data.Src, nil
}
