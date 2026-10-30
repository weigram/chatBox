package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// browserUA is what the app asks pages with: many sites refuse a Go user
// agent, or send one HTML that no browser would get.
const browserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// httpClient fetches the pages and icons the app reads for a chat's name
// and icon.
var httpClient = &http.Client{Timeout: 15 * time.Second}

// pageMeta is what the app recognizes of a web page: its name and its
// icon, which a chat gets from its URL.
type pageMeta struct {
	Title    string
	IconURL  string
	IconData []byte
	IconExt  string
}

// parseURL adds a scheme to a URL the user typed without one, and checks
// that it is a web page.
func parseURL(raw string) (*url.URL, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, errors.New("网址为空")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("网址无效：%w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("只支持 http 和 https 网址")
	}
	if u.Host == "" {
		return nil, errors.New("网址缺少域名")
	}
	return u, nil
}

// detectMeta reads the name and icon of the page at rawURL: its title, the
// icons its HTML links, or the favicon at the root of the site.
func detectMeta(ctx context.Context, rawURL string) (*pageMeta, error) {
	u, err := parseURL(rawURL)
	if err != nil {
		return nil, err
	}
	meta, err := fetchHTML(ctx, u.String())
	if err != nil {
		return nil, err
	}
	if meta.Title == "" {
		meta.Title = hostOf(u.String())
	}
	if meta.IconURL != "" {
		data, ext, err := fetchImage(ctx, meta.IconURL, u.String())
		if err == nil {
			meta.IconData, meta.IconExt = data, ext
		}
	}
	return meta, nil
}

// fetchHTML reads the page at u and parses its title and icons.
func fetchHTML(ctx context.Context, raw string) (*pageMeta, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("网站返回 %s", resp.Status)
	}
	base := resp.Request.URL

	doc, err := html.Parse(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}

	meta := &pageMeta{}
	var icons []iconLink
	var ogTitle, appName string

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				if meta.Title == "" {
					meta.Title = nodeText(n)
				}
			case "meta":
				prop := strings.ToLower(attr(n, "property"))
				name := strings.ToLower(attr(n, "name"))
				switch {
				case prop == "og:title" || prop == "twitter:title":
					ogTitle = attr(n, "content")
				case name == "application-name" || name == "apple-mobile-web-app-title":
					appName = attr(n, "content")
				}
			case "link":
				rel := strings.ToLower(attr(n, "rel"))
				if strings.Contains(rel, "icon") && attr(n, "href") != "" {
					icons = append(icons, iconLink{
						href:  attr(n, "href"),
						rel:   rel,
						sizes: attr(n, "sizes"),
						typ:   strings.ToLower(attr(n, "type")),
					})
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	meta.Title = cleanTitle(meta.Title)
	if meta.Title == "" {
		meta.Title = cleanTitle(ogTitle)
	}
	if meta.Title == "" {
		meta.Title = cleanTitle(appName)
	}
	meta.IconURL = chooseIcon(icons, base)
	return meta, nil
}

// iconLink is a <link rel="...icon..."> of a page.
type iconLink struct {
	href, rel, sizes, typ string
}

// chooseIcon picks the icon to show from those a page links, and the
// site's favicon.ico when it links none: the largest bitmap, then an SVG,
// then anything.
func chooseIcon(icons []iconLink, base *url.URL) string {
	best, bestScore := "", -1
	for _, ic := range icons {
		u, err := base.Parse(strings.TrimSpace(ic.href))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "data") {
			continue
		}
		score := 0
		switch {
		case strings.Contains(ic.typ, "svg") || strings.HasSuffix(u.Path, ".svg"):
			score = 70
		case strings.Contains(ic.typ, "png") || strings.HasSuffix(u.Path, ".png"):
			score = 60
		case strings.Contains(ic.typ, "jpeg") || strings.Contains(ic.typ, "jpg"):
			score = 50
		default:
			score = 30
		}
		if strings.Contains(ic.rel, "apple-touch-icon") {
			score += 15
		}
		if px := iconSize(ic.sizes); px > 0 {
			score += min(px, 512) / 8
		}
		if score > bestScore {
			best, bestScore = u.String(), score
		}
	}
	if best != "" {
		return best
	}
	if base == nil {
		return ""
	}
	return base.Scheme + "://" + base.Host + "/favicon.ico"
}

// iconSize reads the largest size of a sizes attribute, as "32x32", 0 for
// "any" or none.
func iconSize(sizes string) int {
	best := 0
	for _, f := range strings.Fields(strings.ToLower(sizes)) {
		w, _, ok := strings.Cut(f, "x")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(w)
		if err == nil && n > best {
			best = n
		}
	}
	return best
}

// fetchImage downloads an icon, which may be a data: URL.
func fetchImage(ctx context.Context, raw, referer string) ([]byte, string, error) {
	if strings.HasPrefix(raw, "data:") {
		comma := strings.Index(raw, ",")
		if comma < 0 {
			return nil, "", errors.New("data URL 无效")
		}
		payload := raw[comma+1:]
		var data []byte
		var err error
		if strings.Contains(raw[:comma], ";base64") {
			data, err = base64.StdEncoding.DecodeString(payload)
		} else {
			var unescaped string
			unescaped, err = url.PathUnescape(payload)
			data = []byte(unescaped)
		}
		if err != nil {
			return nil, "", err
		}
		return data, "", nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/svg+xml,image/*,*/*;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("图标返回 %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		return nil, "", errors.New("图标为空")
	}
	return data, extOf(raw, data), nil
}

// attr returns the value of n's attribute name.
func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

// nodeText returns the text n holds.
func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}

// cleanTitle makes a page title short and one line, as a dock shows it.
func cleanTitle(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	const max = 40
	if utf8.RuneCountInString(s) > max {
		runes := []rune(s)
		s = strings.TrimSpace(string(runes[:max])) + "…"
	}
	return s
}

// extOf names the format of icon data, for the file the app keeps it in.
func extOf(raw string, data []byte) string {
	switch {
	case len(data) > 8 && bytes.HasPrefix(data, []byte("\x89PNG")):
		return "png"
	case len(data) > 3 && bytes.HasPrefix(data, []byte("\xff\xd8\xff")):
		return "jpg"
	case len(data) > 6 && bytes.HasPrefix(data, []byte("GIF8")):
		return "gif"
	case len(data) > 12 && bytes.HasPrefix(data, []byte("RIFF")) && string(data[8:12]) == "WEBP":
		return "webp"
	case len(data) > 5 && bytes.HasPrefix(data, []byte("BM")):
		return "bmp"
	case len(data) > 4 && bytes.HasPrefix(data, []byte("\x00\x00\x01\x00")):
		return "ico"
	case strings.Contains(string(data[:min(len(data), 256)]), "<svg"):
		return "svg"
	}
	if i := strings.LastIndex(raw, "."); i >= 0 && i < len(raw)-1 {
		ext := strings.ToLower(raw[i+1:])
		if len(ext) <= 4 {
			return ext
		}
	}
	return "bin"
}
