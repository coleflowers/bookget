package app

import (
	"bookget/config"
	"bookget/lib/gohttp"
	"bookget/lib/util"
	"errors"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type KyudbSnu struct {
	dt     *DownloadTask
	itemId string
}

func (k KyudbSnu) Init(iTask int, sUrl string) (msg string, err error) {
	k.dt = new(DownloadTask)
	k.dt.UrlParsed, err = url.Parse(sUrl)
	k.dt.Url = sUrl
	k.dt.Index = iTask
	k.dt.BookId = k.getBookId(k.dt.Url)
	if k.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	k.dt.Jar, _ = cookiejar.New(nil)
	return k.download()
}

func (k KyudbSnu) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`(?i)book_cd=([A-z0-9_-]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (k KyudbSnu) download() (msg string, err error) {
	name := util.GenNumberSorted(k.dt.Index)
	log.Printf("Get %s  %s\n", name, k.dt.Url)
	bs, err := k.getBody(k.dt.Url, k.dt.Jar)
	if err != nil || bs == nil {
		return "requested URL was not found.", err
	}
	match := regexp.MustCompile(`var item_cd = '([A-z0-9_-]+)'`).FindSubmatch(bs)
	if match == nil {
		return "requested URL was not found.", err
	}
	k.itemId = string(match[1])
	respVolume, err := k.getVolumes(k.dt.Url, k.dt.Jar)
	if err != nil {
		return "getVolumes", err
	}
	for i, vol := range respVolume {
		if config.Conf.Volume > 0 && config.Conf.Volume != i+1 {
			continue
		}
		k.dt.SavePath = config.CreateDirectory(k.dt.Url, k.dt.BookId+"_vol."+vol)
		canvases, err := k.getCanvases(vol, k.dt.Jar)
		if err != nil || canvases == nil {
			continue
		}
		log.Printf(" %d/%d volume, %d pages \n", i+1, len(respVolume), len(canvases))
		k.do(canvases)
	}
	return "", nil
}

func (k KyudbSnu) do(imgUrls []string) (msg string, err error) {
	if imgUrls == nil {
		return
	}
	fmt.Println()
	referer := fmt.Sprintf("%s://%s/pf01/rendererImg.do", k.dt.UrlParsed.Scheme, k.dt.UrlParsed.Host)
	for i, uri := range imgUrls {
		if config.SeqContinue(i) {
			continue
		}
		if uri == "" {
			continue
		}
		ext := util.FileExt(uri)
		sortId := util.GenNumberSorted(i + 1)
		log.Printf("Get %d/%d page, URL: %s\n", i+1, len(imgUrls), uri)
		filename := sortId + ext
		dest := k.dt.SavePath + string(os.PathSeparator) + filename
		opts := gohttp.Options{
			DestFile:    dest,
			Overwrite:   false,
			Concurrency: 1,
			CookieFile:  config.Conf.CookieFile,
			CookieJar:   k.dt.Jar,
			Headers: map[string]interface{}{
				"User-Agent": config.Conf.UserAgent,
				"Referer":    referer,
			},
		}
		_, err = gohttp.FastGet(uri, opts)
		if err != nil {
			fmt.Println(err)
			break
		}
	}
	fmt.Println()
	return "", err
}
func (k KyudbSnu) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	d := map[string]interface{}{
		"item_cd":       k.itemId,
		"book_cd":       k.dt.BookId,
		"vol_no":        "",
		"page_no":       "",
		"imgFileNm":     "",
		"tbl_conts_seq": "",
		"mokNm":         "",
		"add_page_no":   "",
	}
	cli := gohttp.NewClient(gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent":   config.Conf.UserAgent,
			"Referer":      url.PathEscape(sUrl),
			"Content-Type": "application/x-www-form-urlencoded",
		},
		FormParams: d,
	})
	resp, err := cli.Post(fmt.Sprintf("%s://%s/pf01/rendererImg.do", k.dt.UrlParsed.Scheme, k.dt.UrlParsed.Host))
	bs, err := resp.GetBody()
	if bs == nil || err != nil {
		return nil, err
	}
	matches := regexp.MustCompile(`<option\s+value=["']([A-z0-9]+)["']`).FindAllSubmatch(bs, -1)
	if matches == nil {
		err = errors.New("requested URL was not found.")
		return nil, err
	}
	for _, m := range matches {
		volumes = append(volumes, string(m[1]))
	}
	return volumes, nil
}

func (k KyudbSnu) getCanvases(vol string, jar *cookiejar.Jar) (canvases []string, err error) {
	sUrl := fmt.Sprintf("%s://%s/pf01/rendererImg.do", k.dt.UrlParsed.Scheme, k.dt.UrlParsed.Host)
	d := map[string]interface{}{
		"item_cd": k.itemId,
		"book_cd": k.dt.BookId,
		"vol_no":  vol,
		"page_no": "",
		"tool":    "1",
	}
	cli := gohttp.NewClient(gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent":   config.Conf.UserAgent,
			"Referer":      url.PathEscape(sUrl),
			"Content-Type": "application/x-www-form-urlencoded",
		},
		FormParams: d,
	})
	resp, err := cli.Post(sUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	if bs == nil {
		err = errors.New(resp.GetReasonPhrase())
		return nil, err
	}
	var fromPage string
	m := regexp.MustCompile(`first_page_no\s+=\s+['"]([A-z0-9]+)['"];`).FindSubmatch(bs)
	if m != nil {
		fromPage = string(m[1])
	}
	var pageId string
	m = regexp.MustCompile(`imgFileNm\s+=\s+['"]([^"']+)['"]`).FindSubmatch(bs)
	if m != nil {
		pageId = string(m[1])
	}
	imgFileNm := filepath.Base(pageId)
	matches := regexp.MustCompile(`onclick="fn_goPageJump\('([A-z0-9]+)'\);">([A-z0-9]+)</a>`).FindAllSubmatch(bs, -1)
	_fromPage := vol + "_" + fromPage
	for _, match := range matches {
		_page := vol + "_" + string(match[1])
		_imgFileNm := strings.ReplaceAll(imgFileNm, _fromPage, _page)
		_pageId := strings.ReplaceAll(pageId, _fromPage, _page)
		//imgUrl := fmt.Sprintf("%s://%s/ImageDown.do?imgFileNm=%s&path=%s", k.dt.UrlParsed.Scheme, k.dt.UrlParsed.Host, _imgFileNm, _pageId)
		imgUrl := fmt.Sprintf("%s://%s/ImageServlet.do?imgFileNm=%s&path=%s", k.dt.UrlParsed.Scheme, k.dt.UrlParsed.Host, _imgFileNm, _pageId)
		canvases = append(canvases, imgUrl)
	}
	return canvases, nil
}

func (k KyudbSnu) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
	cli := gohttp.NewClient(gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
		},
	})
	resp, err := cli.Get(apiUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	if bs == nil {
		err = errors.New(resp.GetReasonPhrase())
		return nil, err
	}
	return bs, nil
}
