package main

import (
	"log"
	"net/url"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

// defaultSearchURL searches the web for what the app asks it to: "%s" is
// the words, quoted. The settings change it.
const defaultSearchURL = "https://cn.bing.com/search?q=%s"

// pageActions are what a page menu's items do to a page, which *mygo.Page
// is: saying it this way leaves the building of the menu testable without
// a page.
type pageActions interface {
	Reload()
	GoBack()
	GoForward()
	Print()
	OpenDevTools()
}

// pageMenu builds the menu a chat's page shows on a right-click: the web
// view's own items stay where they are, and the app adds what a browser's
// page offers above and below them.
func (a *app) pageMenu(e *mygo.ContextMenuEvent, ch *Chat, v *mygo.WebView) {
	if v == nil || v.IsDestroyed() {
		return
	}
	a.pageMenuFor(e, ch, v.Page())
}

// pageMenuFor is pageMenu on the actions of a page.
func (a *app) pageMenuFor(e *mygo.ContextMenuEvent, ch *Chat, page pageActions) {
	menu := e.Menu
	if menu == nil {
		return
	}
	pageURL := a.pageURL[ch.ID]

	// Above: going back and forward, and reloading, as a browser's menu
	// begins. WebKit shows them itself most of the time; these are for
	// when it does not.
	var top []*mygo.MenuItem
	if menu.ItemByID(mygo.ContextMenuBack) == nil {
		top = append(top, menuItem("mychatbox.back", "后退", func() { page.GoBack() }))
	}
	if menu.ItemByID(mygo.ContextMenuForward) == nil {
		top = append(top, menuItem("mychatbox.forward", "前进", func() { page.GoForward() }))
	}
	if menu.ItemByID(mygo.ContextMenuReload) == nil {
		top = append(top, menuItem("mychatbox.reload", "重新载入", func() { page.Reload() }))
	}
	for i, it := range top {
		menu.Insert(i, it)
	}
	if len(top) > 0 {
		menu.Insert(len(top), mygo.Separator())
	}

	// Below: what a page of a browser offers that the web view's own menu
	// does not, the selection and the link clicked first.
	var tail []*mygo.MenuItem
	if selection := strings.TrimSpace(e.SelectionText); selection != "" {
		if menu.ItemByID(mygo.ContextMenuCopy) == nil {
			tail = append(tail, menuItem("mychatbox.copy", "复制选中文字",
				func() { a.copyText(selection) }))
		}
		tail = append(tail, menuItem("mychatbox.copySelectionLink", "复制所选中内容的链接",
			func() { a.copyText(textFragment(pageURL, selection)) }))
		tail = append(tail, menuItem("mychatbox.search",
			"在浏览器中搜索“"+shorten(selection, 20)+"”",
			func() { a.openInBrowser(a.searchURL(selection)) }))
	}
	if e.LinkURL != "" {
		link := e.LinkURL
		if menu.ItemByID(mygo.ContextMenuCopyLink) == nil {
			tail = append(tail, menuItem("mychatbox.copyLink", "复制链接地址",
				func() { a.copyText(link) }))
		}
		tail = append(tail, menuItem("mychatbox.openLink", "用默认浏览器打开链接",
			func() { a.openInBrowser(link) }))
	}
	if pageURL != "" {
		tail = append(tail, menuItem("mychatbox.copyPage", "复制页面地址",
			func() { a.copyText(pageURL) }))
		tail = append(tail, menuItem("mychatbox.openPage", "用默认浏览器打开当前页",
			func() { a.openInBrowser(pageURL) }))
	}
	if menu.ItemByID(mygo.ContextMenuPrint) == nil {
		tail = append(tail, menuItem("mychatbox.print", "打印…", func() { page.Print() }))
	}
	if menu.ItemByID(mygo.ContextMenuInspect) == nil {
		tail = append(tail, menuItem("mychatbox.inspect", "检查元素", func() { page.OpenDevTools() }))
	}
	if len(tail) > 0 {
		menu.Append(append([]*mygo.MenuItem{mygo.Separator()}, tail...)...)
	}
	if envVar("DEBUG_MENU") != "" {
		log.Printf("debug: page menu of %s: %v", ch.Name, labelsList(menu))
	}
}

// labelsList names a menu's items, for logs and for what a failing test
// prints.
func labelsList(m *mygo.Menu) []string {
	var out []string
	for _, it := range m.Items() {
		if it.Type == mygo.MenuItemSeparator {
			out = append(out, "---")
			continue
		}
		out = append(out, it.Label)
	}
	return out
}

// menuItem makes one of the app's page menu items, whose work runs once
// the menu closed.
func menuItem(id, label string, click func()) *mygo.MenuItem {
	return &mygo.MenuItem{
		ID:    id,
		Label: label,
		Click: func(*mygo.MenuItem, *mygo.Window) { click() },
	}
}

// copyText puts text on the clipboard.
func (a *app) copyText(text string) {
	if text == "" {
		return
	}
	if err := mygo.Clipboard.Write(transfer.TextData(text)); err != nil {
		a.toast("复制失败：" + err.Error())
	}
}

// openInBrowser shows a URL in the user's browser.
func (a *app) openInBrowser(raw string) {
	if raw == "" {
		return
	}
	if err := mygo.Shell.OpenExternal(raw); err != nil {
		a.toast("打不开浏览器：" + err.Error())
	}
}

// searchURL makes the URL that searches the web for text, from the
// template the settings hold.
func (a *app) searchURL(text string) string {
	tmpl := strings.TrimSpace(a.store.SearchURL)
	if tmpl == "" || !strings.Contains(tmpl, "%s") {
		tmpl = defaultSearchURL
	}
	return strings.ReplaceAll(tmpl, "%s", url.QueryEscape(text))
}

// textFragment makes the link to a page that lands on text, as Safari's
// "Copy Link to Selected Content" does: the page's URL with a text
// fragment, which browsers that know it highlight.
func textFragment(pageURL, text string) string {
	text = strings.TrimSpace(text)
	if pageURL == "" || text == "" {
		return pageURL
	}
	if u, err := url.Parse(pageURL); err == nil {
		u.Fragment = ""
		pageURL = u.String()
	}
	// A text fragment is a URL fragment: spaces go as %20, not as +.
	quoted := strings.ReplaceAll(url.QueryEscape(limitRunes(text, 120)), "+", "%20")
	return pageURL + "#:~:text=" + quoted
}

// shorten trims text to n runes, with an ellipsis when it cut any.
func shorten(text string, n int) string {
	trimmed := limitRunes(text, n)
	if trimmed == text {
		return trimmed
	}
	return trimmed + "…"
}

// limitRunes returns the first n runes of text.
func limitRunes(text string, n int) string {
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[:n])
}
