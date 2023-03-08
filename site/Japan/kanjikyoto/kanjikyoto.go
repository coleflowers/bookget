package kanjikyoto

import (
	"bookget/config"
	curl "bookget/lib/curl"
	util "bookget/lib/util"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
)

func Init(iTask int, taskUrl string) (msg string, err error) {
	bookId := ""
	//m := regexp.MustCompile(`/html/([A-Za-z0-9_-]+).html`).FindStringSubmatch(taskUrl)
	m := regexp.MustCompile(`/html/([A-Za-z0-9_-]+)menu.html`).FindStringSubmatch(taskUrl)
	if m != nil {
		bookId = m[1]
		config.CreateDirectory(taskUrl, bookId)
		StartDownload(iTask, taskUrl, bookId)
	}
	return "", err
}

func StartDownload(iTask int, taskUrl, bookId string) {
	name := util.GenNumberSorted(iTask)
	log.Printf("Get %s  %s\n", name, taskUrl)

	bookUrls, err := getMultiplebooks(taskUrl)
	if err != nil {
		return
	}
	size := len(bookUrls)
	imageUrls, e := getImages(bookUrls[size-1])
	if e != nil {
		return
	}
	size = len(imageUrls)
	log.Printf(" %d pages.\n", size)
	for i, uri := range imageUrls {
		if config.SeqContinue(i) {
			continue
		}
		if uri == "" {
			continue
		}
		ext := util.FileExt(uri)
		sortId := util.GenNumberSorted(i + 1)
		log.Printf("Get %s  %s\n", sortId, uri)
		fileName := sortId + ext
		dest := config.GetDestPath(taskUrl, bookId, fileName)
		curl.FastGet(uri, dest, nil, true)
	}
	return
}

func getMultiplebooks(taskUrl string) (bookUrls []string, err error) {
	bs, err := curl.Get(taskUrl, nil)
	if err != nil {
		return
	}
	text := string(bs)
	//取册数
	matches := regexp.MustCompile(`href=["']?(.+?)\.html["']?`).FindAllStringSubmatch(text, -1)
	if matches == nil {
		return
	}
	pos := strings.LastIndex(taskUrl, "/")
	hostUrl := taskUrl[:pos]
	links := make([]string, 0, len(matches))
	for _, v := range matches {
		if strings.Contains(v[1], "top") {
			continue
		}
		s := fmt.Sprintf("%s/%s.html", hostUrl, v[1])
		links = append(links, s)
	}

	return links, err
}

func getImages(volumeUrl string) (imageUrls []string, err error) {
	bs, err := curl.Get(volumeUrl, nil)
	if err != nil {
		return
	}
	text := string(bs)

	startPos, ok := getVolStartPos(&text)
	if !ok {
		return
	}
	maxPage, ok := getVolMaxPage(&text)
	if !ok {
		return
	}
	bookNumber, ok := getBookNumber(&text)
	if !ok {
		return
	}
	//curPage, _ := getVolCurPage(&text)
	//if !ok {
	//	return
	//}
	pos := strings.LastIndex(volumeUrl, "/")
	pos1 := strings.LastIndex(volumeUrl[:pos], "/")
	hostUrl := volumeUrl[:pos1]
	maxPos := startPos + maxPage
	for i := 1; i < maxPos; i++ {
		sortId := util.GenNumberSorted(i)
		imgUrl := fmt.Sprintf("%s/L/%s%s.jpg", hostUrl, bookNumber, sortId)
		imageUrls = append(imageUrls, imgUrl)
	}
	return
}
func getBookNumber(text *string) (bookNumber string, ok bool) {
	//当前开始位置
	match := regexp.MustCompile(`var[\s]+bookNum[\s]+=["'\s]*([A-z0-9]+)["'\s]*;`).FindStringSubmatch(*text)
	if match == nil {
		return "", false
	}
	return match[1], true
}

func getVolStartPos(text *string) (startPos int, ok bool) {
	//当前开始位置
	match := regexp.MustCompile(`var[\s]+volStartPos[\s]*=[\s]*([0-9]+)[\s]*;`).FindStringSubmatch(*text)
	if match == nil {
		return 0, false
	}
	startPos, _ = strconv.Atoi(match[1])
	return startPos, true
}

func getVolCurPage(text *string) (curPage int, ok bool) {
	//当前开始位置
	match := regexp.MustCompile(`var[\s]+curPage[\s]*=[\s]*([0-9]+)[\s]*;`).FindStringSubmatch(*text)
	if match == nil {
		return 0, false
	}
	curPage, _ = strconv.Atoi(match[1])
	return curPage, true
}

func getVolMaxPage(text *string) (maxPage int, ok bool) {
	//当前开始位置
	match := regexp.MustCompile(`var[\s]+volMaxPage[\s]*=[\s]*([0-9]+)[\s]*;`).FindStringSubmatch(*text)
	if match == nil {
		return 0, false
	}
	maxPage, _ = strconv.Atoi(match[1])
	return maxPage, true
}
