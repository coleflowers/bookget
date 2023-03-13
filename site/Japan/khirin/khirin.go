package khirin

import (
	"bookget/app"
	"bookget/config"
	curl "bookget/lib/curl"
	util "bookget/lib/util"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
)

func Init(iTask int, taskUrl string) (msg string, err error) {
	//StartDownload(iTask, taskUrl, bookId)
	taskUrls := explanRegexpUrl(taskUrl)
	taskName := util.GenNumberSorted(iTask)
	log.Printf("Get %s  %s\n", taskName, taskUrl)
	for i, tUrl := range taskUrls {
		if config.SeqContinue(i) {
			continue
		}
		bookId := getBookId(tUrl)
		if bookId == "" {
			continue
		}
		config.CreateDirectory(taskUrl, bookId)
		name := util.GenNumberSorted(i + 1)
		log.Printf("Get %s  %s\n", name, tUrl)
		startDownload(tUrl, bookId)
	}
	return "", err
}

func getBookId(taskUrl string) string {
	//m := regexp.MustCompile(`ac.jp/database/([A-z\d_-]+)`).FindStringSubmatch(taskUrl)
	//if m != nil {
	//	return m[1]
	//}
	m := regexp.MustCompile(`ac.jp/([A-z\d_-]+)/([A-z\d_-]+)`).FindStringSubmatch(taskUrl)
	if m != nil {
		return fmt.Sprintf("%s.%s", m[1], m[2])
	}
	return ""
}

func explanRegexpUrl(taskUrl string) (taskUrls []string) {
	uriMatch, ok := util.GetUriMatch(taskUrl)
	if ok {
		iMinLen := len(uriMatch.Min)
		for i := uriMatch.IMin; i <= uriMatch.IMax; i++ {
			iLen := len(strconv.Itoa(i))
			if iLen < iMinLen {
				iLen = iMinLen
			}
			sortId := util.GenNumberLimitLen(i, iLen)
			dUrl := regexp.MustCompile(`\((\d+)-(\d+)\)`).ReplaceAll([]byte(taskUrl), []byte(sortId))
			sUrl := string(dUrl)
			taskUrls = append(taskUrls, sUrl)
		}
		return
	}
	taskUrls = append(taskUrls, taskUrl)
	return
}

func startDownload(pageUrl, bookId string) {
	canvases := getCanvases(pageUrl)
	if canvases.Size == 0 {
		return
	}
	log.Printf(" %d pages.\n", canvases.Size)
	config.CreateDirectory(pageUrl, bookId)
	if config.Conf.UseDziRs {
		app.DziDownload(pageUrl, bookId, canvases.IiifUrls)
	} else {
		app.NormalDownload(pageUrl, bookId, canvases.ImgUrls, nil)
	}
}

func getManifestUrl(pageUrl string) (uri string, err error) {
	bs, err := curl.Get(pageUrl, nil)
	if err != nil {
		return
	}
	text := string(bs)
	//<iframe id="uv-iframe" class="uv-iframe" src="/libraries/uv/uv.html#?manifest=/iiif/rekihaku/H-173-1/manifest.json"></iframe>
	m := regexp.MustCompile(`manifest=(.+?)["']`).FindStringSubmatch(text)
	if m == nil {
		return
	}
	if !strings.HasPrefix(m[1], "https://") {
		uri = fmt.Sprintf("https://khirin-a.rekihaku.ac.jp%s", m[1])
	} else {
		uri = m[1]
	}
	return
}

func getCanvases(pageUrl string) (canvases Canvases) {
	uri, err := getManifestUrl(pageUrl)
	if err != nil {
		return
	}
	bs, err := curl.Get(uri, nil)
	if err != nil {
		return
	}
	var manifest = new(Manifest)
	if err = json.Unmarshal(bs, manifest); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	if len(manifest.Sequences) == 0 {
		return
	}
	newWidth := ""
	//>6400使用原图
	if config.Conf.FullImageWidth > 6400 {
		newWidth = "full/full"
	} else if config.Conf.FullImageWidth >= 1000 {
		newWidth = fmt.Sprintf("full/%d,", config.Conf.FullImageWidth)
	}

	size := len(manifest.Sequences[0].Canvases)
	canvases.ImgUrls = make([]string, 0, size)
	canvases.IiifUrls = make([]string, 0, size)
	for _, canvase := range manifest.Sequences[0].Canvases {
		for _, image := range canvase.Images {
			//iifUrl, _ := url.QueryUnescape(image.Resource.Service.Id)
			//操蛋的网站，它有错误
			if strings.Contains(image.Resource.Service.Id, "[paragraph:field_filepath]") {
				continue
			}
			//dezoomify-rs URL
			iiiInfo := fmt.Sprintf("%s/info.json", image.Resource.Service.Id)
			canvases.IiifUrls = append(canvases.IiifUrls, iiiInfo)

			//JPEG URL
			imgUrl := fmt.Sprintf("%s/%s/0/default.jpg", image.Resource.Service.Id, newWidth)
			canvases.ImgUrls = append(canvases.ImgUrls, imgUrl)
		}
	}
	canvases.Size = size
	return
}
