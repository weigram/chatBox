package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// app is the state the window shows: the chats, the one open, and the
// dialogs over it.
type app struct {
	win     *mygo.Window
	store   *Store
	icons   *iconStore
	dataDir string

	// views are the web views of the chats open, by chat ID. One the app
	// did not open yet has none.
	views map[string]*mygo.WebView
	// pageURL is the address each chat's page was at last, which its
	// context menu copies and opens: the menu shows while the page waits,
	// when asking the page itself would hold it.
	pageURL map[string]string
	// sessions are the isolated sessions of chats that keep their website
	// data apart.
	sessions map[string]*mygo.Session

	library bool

	// settings is the settings dialog; setShortcut and setShortcutOn are
	// the shortcut it edits, before it is saved.
	settings      bool
	setShortcut   string
	setShortcutOn bool
	setSearch     string
	setToolbar    bool
	// recording is the shortcut recorder listening for keys; recError says
	// what went wrong with the last one, and recPending the accelerator a
	// conflict asks about.
	// addrText is what the address bar shows, and addrFocused whether it is
	// open for typing: while it is, the page's own address stops
	// overwriting what the user types.
	addrText    string
	addrFocused bool

	// dragTab is the tab being dragged along the strip, dragTabDX how far it
	// has gone, and tabRects where the tabs sat when the frame was built:
	// the drop takes the one the pointer is over.
	dragTab     string
	dragTabFrom float32
	// tabRects are the tabs' places from the frame before, which a drop
	// looks at: this frame's are still being filled while it happens.
	tabRects  []tabRect
	nextRects []tabRect

	// dragChat is the pinned chat being dragged up or down the dock, and
	// dragDY how far it has gone: Dragged reports one frame at a time.
	dragChat string
	dragDY   float32

	// quitting is set once the app is on its way out, which lets the
	// window close: a close any other time only puts it away.
	quitting bool

	recording       bool
	recFocused      bool
	recError        string
	recPending      string
	confirmShortcut bool
	// confirmForEnable tells the question apart: one asked as a key is
	// recorded, and one asked as the shortcut is turned on.
	confirmForEnable bool
	settingsMsg      string
	settingsErr      bool
	// shortcutDone is the accelerator the app registered, empty for none.
	shortcutDone string

	editor    bool
	editorTab int
	draft     draft
	editingID string

	detectSeq int

	// iconMu guards iconTried: the chats whose icon the app looked for.
	iconMu    sync.Mutex
	iconTried map[string]bool

	pendingToast string
}

// The editor's tabs, in the order it shows them.
const (
	tabBasic = iota
	tabProxy
	tabCSS
	tabJS
)

// draft is the chat the editor edits, before it is saved.
type draft struct {
	ID        string
	Name      string
	URL       string
	CSS       string
	JS        string
	UserAgent string
	Zoom      string
	ProxyMode string
	ProxyURL  string
	Bypass    string
	Isolated  bool
	AllFrames bool
	IconName  string
	IconPic   ui.ImageSource

	Detecting bool
	Message   string
	Error     string
	Advanced  bool
}

// newApp loads the chats of the data directory.
func newApp() (*app, error) {
	dir, err := resolveDataDir()
	if err != nil {
		return nil, err
	}
	store, err := loadStore(dir)
	if err != nil {
		return nil, err
	}
	return &app{
		store:     store,
		icons:     newIconStore(filepath.Join(dir, "icons")),
		dataDir:   dir,
		views:     map[string]*mygo.WebView{},
		pageURL:   map[string]string{},
		sessions:  map[string]*mygo.Session{},
		iconTried: map[string]bool{},
	}, nil
}

// resolveDataDir returns the directory the app keeps its chats in:
// CHATBOX_HOME (or the MYCHATBOX_HOME the app read under its old name)
// when the user set it, the app's user data directory otherwise. The
// directory the app had under that old name is moved there the first time,
// so that renaming the app costs no chats.
func resolveDataDir() (string, error) {
	if dir := strings.TrimSpace(envVar("HOME")); dir != "" {
		return dir, os.MkdirAll(dir, 0o755)
	}
	var dir string
	if d, err := mygo.App.Path(mygo.PathUserData); err == nil && d != "" {
		dir = d
	} else {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "chatBox")
	}
	moveLegacyDataDir(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return dir, err
	}
	return dir, nil
}

// envVar reads CHATBOX_<name>, and the MYCHATBOX_<name> an older build read,
// so that a script or a shell profile keeps working across the rename.
func envVar(name string) string {
	if v := os.Getenv("CHATBOX_" + name); v != "" {
		return v
	}
	return os.Getenv("MYCHATBOX_" + name)
}

// moveLegacyDataDir moves the directory the app kept before it was renamed
// to the one it uses now, once: without it, a rename would look like a fresh
// install, chats and all.
func moveLegacyDataDir(dir string) {
	const legacyName = "MyChatBox"
	if dir == "" || filepath.Base(dir) == legacyName {
		return
	}
	legacy := filepath.Join(filepath.Dir(dir), legacyName)
	if _, err := os.Stat(dir); err == nil {
		return
	}
	if _, err := os.Stat(legacy); err != nil {
		return
	}
	if err := os.Rename(legacy, dir); err != nil {
		log.Printf("chatBox: 老数据目录没搬过来（%v），可以从 %s 手动拷到 %s", err, legacy, dir)
	}
}

// start opens the app's window, and the chat the app showed last time.
func (a *app) start() {
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:       "chatBox",
		Width:       1240,
		Height:      820,
		MinWidth:    780,
		MinHeight:   520,
		StateKey:    "main",
		AlwaysOnTop: a.store.AlwaysOnTop,
		// 标签栏自己画在窗口最上面，交通灯浮在它左侧；窗口上沿的空白
		// 用来拖动窗口。
		TitleBarStyle:   mygo.TitleBarHidden,
		BackgroundColor: "light-dark(#f5f5f7, #1b1b1d)",
		Content:         ui.View(a.view),
	})
	// 关窗口只是把窗口收起来，应用继续跑着——否则窗口一关，进程跟着退出，
	// 全局快捷键自然就没了。真正退出走 ⌘Q（或应用菜单里的「退出」）。
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) { a.quitting = true })
	a.win.OnClose(func(e *mygo.CloseEvent) {
		if a.quitting {
			return
		}
		e.PreventDefault()
		a.win.Hide()
	})
	a.registerShortcut()
	// The window comes back when the app is activated with none showing,
	// as a click on the Dock icon after the shortcut hid it.
	mygo.App.OnActivate(func(hasVisibleWindows bool) {
		if !hasVisibleWindows && a.win != nil && !a.win.IsVisible() {
			a.win.Show()
		}
	})
	if id := a.store.Active; id != "" && a.store.IsOpen(id) {
		a.activate(id)
	}
	if debugWanted() {
		a.debugRun()
	}
}

// toggleWindow shows the app's window, or hides it while it is the one
// the user is looking at. It is what the global shortcut does.
func (a *app) toggleWindow() {
	if a.win == nil {
		return
	}
	// Visible and in front: the key puts it away. Otherwise it comes up:
	// hidden, under other windows, or in the Dock.
	if a.win.IsVisible() && !a.win.IsMinimized() && mygo.App.IsActive() {
		a.win.Hide()
		return
	}
	mygo.App.Show()
	a.win.Show()
	if a.win.IsMinimized() {
		a.win.Restore()
	}
	mygo.App.Focus()
	a.win.Focus()
}

// registerShortcut claims the global shortcut of the settings, and lets go
// of the one claimed before. It returns what mygo says when the system
// refuses it, as when another application holds the keys.
func (a *app) registerShortcut() error {
	if a.win == nil {
		return nil
	}
	if a.shortcutDone != "" {
		mygo.GlobalShortcut.Unregister(a.shortcutDone)
		a.shortcutDone = ""
	}
	acc := strings.TrimSpace(a.store.Shortcut)
	if !a.store.ShortcutOn || acc == "" {
		return nil
	}
	if err := mygo.GlobalShortcut.Register(acc, a.toggleWindow); err != nil {
		return err
	}
	a.shortcutDone = acc
	return nil
}

// applyAlwaysOnTop keeps the window above the other applications', and
// remembers the choice.
func (a *app) applyAlwaysOnTop() {
	if a.win != nil {
		a.win.SetAlwaysOnTop(a.store.AlwaysOnTop)
	}
	if err := a.store.Save(); err != nil {
		log.Printf("myChatBox: 保存设置失败：%v", err)
	}
	if a.store.AlwaysOnTop {
		a.toast("窗口已固定在最前")
	} else {
		a.toast("窗口不再固定在最前")
	}
}

// openSettings opens the settings dialog on what the app has stored.
func (a *app) openSettings() {
	a.setShortcut = a.store.Shortcut
	a.setShortcutOn = a.store.ShortcutOn
	a.setSearch = a.store.SearchURL
	a.setToolbar = a.store.ShowToolbar
	a.recording, a.recError, a.recPending = false, "", ""
	a.confirmShortcut, a.confirmForEnable = false, false
	a.settingsMsg, a.settingsErr = "", false
	a.library, a.editor = false, false
	a.settings = true
}

// saveSettings keeps the shortcut the dialog edits, and registers it. It
// keeps the dialog open, with what went wrong, when the system refuses it.
func (a *app) saveSettings() {
	a.recording = false
	a.store.Shortcut = strings.TrimSpace(a.setShortcut)
	a.store.ShortcutOn = a.setShortcutOn && a.store.Shortcut != ""
	a.store.SearchURL = strings.TrimSpace(a.setSearch)
	a.store.ShowToolbar = a.setToolbar
	if err := a.registerShortcut(); err != nil {
		a.settingsMsg, a.settingsErr = err.Error(), true
		return
	}
	a.settingsMsg, a.settingsErr = "", false
	if err := a.store.Save(); err != nil {
		log.Printf("myChatBox: 保存设置失败：%v", err)
	}
	a.settings = false
	a.toast("设置已保存")
}

// view builds the window: the dock, the chat open, and the dialogs over
// them.
func (a *app) view(c *ui.Context) {
	if a.pendingToast != "" {
		c.Toast(a.pendingToast)
		a.pendingToast = ""
	}
	root := ui.Column(c).Fill().AlignItems(ui.Stretch)
	root.OnShortcut(ui.Cmd, ui.KeyN, func() { a.openEditor("", tabBasic) })
	root.OnShortcut(ui.Cmd, ui.KeyL, func() { a.library = true })
	root.Children(func() {
		a.titleBar(c)
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).MinHeight(0).Children(func() {
			a.dock(c)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				if a.store.ShowToolbar {
					a.toolbar(c)
				}
				a.content(c)
			})
		})
	})
	a.libraryView(c)
	a.editorView(c)
	a.settingsView(c)
	a.confirmShortcutView(c)
}

// dock builds the chats open, in a column at the left of the window.
func (a *app) dock(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(dockWidth).FillHeight().Padding(0, 4).Gap(4).
		AlignItems(ui.Center).Background(t.Surface).Children(func() {
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(4).AlignItems(ui.Center).Children(func() {
				// 固定的聊天在上面，一直显示；其余的是当前打开的，
				// 和固定的一组之间有一条细线分开。
				pinned, rest := a.store.DockChats()
				for _, ch := range pinned {
					a.dockItem(c, ch)
				}
				if len(pinned) > 0 && len(rest) > 0 {
					ui.Row(c).Width(dockRing-8).Height(1).Margin(0, 2).
						Background(t.Border)
				}
				for _, ch := range rest {
					a.dockItem(c, ch)
				}
			})
		})
		// 底部只剩设置：新增聊天在标签栏的 ＋ 与 ⌘N 里。
		dockIconButton(c, "设置：全局快捷键、地址栏与数据目录", gearIcon, a.openSettings)
	})
}

// dockItem builds one chat of the dock: its icon, highlighted while it is
// the chat the app shows.
// dockItem builds one chat of the dock. The row it takes up is roomy and
// stays unpainted, so that a click anywhere on it works: what a chat is
// the one shown is told by the ring around its icon, not by a block of
// color across the row, as Cherry Studio draws it.
func (a *app) dockItem(c *ui.Context, ch *Chat) {
	t := c.Theme()
	active := ch.ID == a.store.Active
	row := ui.Row(c).Key("dock:" + ch.ID).Width(40).Height(dockItemHeight).
		AlignItems(ui.Center).Justify(ui.Center)
	if row.Clicked() {
		a.activate(ch.ID)
	}
	// 固定的聊天可以上下拖着换位置：拖的时候跟手，松手后按拖了几格插回去。
	// 没固定的那些顺序就是打开顺序，不给拖。
	// Dragged 报的是“这一帧挪了多少”，所以要自己累加。
	if ch.Pinned {
		_, dy, dragging := row.Dragged()
		_, _, ended := row.DragEnded()
		switch {
		case dragging:
			if a.dragChat != ch.ID {
				a.dragChat, a.dragDY = ch.ID, 0
			}
			a.dragDY += dy
			row.Translate(0, a.dragDY)
			row.Cursor(ui.CursorGrabbing)
		case ended && a.dragChat == ch.ID:
			steps := int(a.dragDY/(dockItemHeight+dockGap) + sign(a.dragDY)*0.5)
			a.dragChat, a.dragDY = "", 0
			if a.store.MovePinned(ch.ID, steps) {
				a.saveStore()
				if a.win != nil {
					a.win.Invalidate()
				}
			}
		case !row.Pressed() && a.dragChat == ch.ID:
			// 松手发生在别处，没算成一次拖拽：把累积清掉。
			a.dragChat, a.dragDY = "", 0
			row.Cursor(ui.CursorGrab)
		default:
			row.Cursor(ui.CursorGrab)
		}
	}
	row.Label("打开 " + ch.Name)
	row.Tooltip(ch.Name)
	row.ContextMenu(func(m *ui.Menu) { a.chatMenu(m, ch) })
	row.Children(func() {
		// 每个图标都坐一个同样大小的白圆片；选中的那个是纯白并带一点投影，
		// 其余是半透明的白，指针停上去时更白一些。选中不放大，大小始终一致。
		ring := ui.Row(c).Size(dockRing, dockRing).Radius(dockRing / 2).
			AlignItems(ui.Center).Justify(ui.Center)
		ring.Background(iconChip(t, active, row.Hovered()))
		if active {
			ring.Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.12))
		}
		ring.Children(func() {
			a.avatar(c, ch, dockIcon)
		})
	})
}

// The dock's sizes: the ring around the icon of the chat shown is bigger,
// its icon with it, and that is what tells the chat is the one showing.
// One size for every chat's circle, the chosen one's included: it is the
// white of it that says which chat shows, not its size. The sizes leave the
// circle 4 DIPs in from the dock's edges, which is what the column's
// padding gives it: a circle wider than that is cut.
const (
	dockWidth = float32(48)
	dockRing  = float32(32)
	dockIcon  = float32(22)
	// dockItemHeight and dockGap make the pitch a drag is measured in.
	dockItemHeight = float32(40)
	dockGap        = float32(4)
)

// titleBar builds the window's top row: the chats open as a browser's
// tabs, in the room the hidden title bar leaves. The traffic lights show
// over its left edge, which the tabs keep clear of, and the row itself
// drags the window where no tab takes the pointer.
func (a *app) titleBar(c *ui.Context) {
	t := c.Theme()
	bar := c.TitleBar()
	// As tall as the band the window controls take, so that they sit in
	// the middle of the row, and never shorter than a tab.
	height := bar.Height
	if height < 30 {
		height = 30
	}
	// 窗口自己的按钮占哪一侧，置顶就放另一侧：macOS 的交通灯在左，
	// Windows 的最小化 / 最大化 / 关闭在右。两侧都留出它们占的宽度。
	pin := func() {
		circleIconButton(c, "窗口固定在最前（置顶）", pinIcon, a.store.AlwaysOnTop,
			22, 12, func() {
				a.store.AlwaysOnTop = !a.store.AlwaysOnTop
				a.applyAlwaysOnTop()
			})
	}
	tabs := func() {
		ui.ScrollHorizontal(c).Grow(1).Children(func() {
			ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
				a.nextRects = a.nextRects[:0]
				for _, ch := range a.store.OpenChats() {
					a.tabItem(c, ch)
				}
				a.tabRects = append(a.tabRects[:0], a.nextRects...)
				plus := ui.Row(c).Size(26, 26).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
				plus.Label("新增聊天")
				plus.Tooltip("新增聊天")
				if plus.Hovered() {
					plus.Background(t.SurfaceHover)
				}
				if plus.Clicked() {
					a.library = true
				}
				plus.Children(func() {
					ui.Icon(c, plusIcon).Size(15, 15).TextColor(t.TextMuted)
				})
			})
		})
	}
	// 整条底是浅灰（与 Dock 同色），选中的标签是它上面的一块白，像
	// Cherry Studio 那样：白块从灰底上浮起来，而不是反过来。
	row := ui.Row(c).Height(height).
		Padding(0, bar.Right+6, 0, bar.Left+6).
		Gap(6).AlignItems(ui.Center).
		Background(t.Surface)
	row.DragWindow()
	row.Children(func() {
		if pinLeads(bar) {
			pin()
		}
		tabs()
		if !pinLeads(bar) {
			pin()
		}
	})
}

// pinLeads reports whether the always-on-top button belongs at the start of
// the title bar: the window's own buttons take the right side on Windows, and
// the left side on macOS, and the pin goes where they are not.
func pinLeads(bar ui.TitleBar) bool {
	return bar.Right > bar.Left
}

// tabItem builds one tab: the chat's icon and name, which a click shows,
// and the close button beside them.
func (a *app) tabItem(c *ui.Context, ch *Chat) {
	t := c.Theme()
	active := ch.ID == a.store.Active
	tab := ui.Row(c).Key("tab:"+ch.ID).Height(26).Radius(7).Padding(3, 7).Gap(6).
		AlignItems(ui.Center)
	switch {
	case active:
		tab.Background(chip(t))
		tab.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.10))
	case tab.Hovered():
		tab.Background(hoverWash(t))
	}
	tab.Tooltip(ch.Name + "\n" + describeProxyFully(ch.Proxy))
	tab.ContextMenu(func(m *ui.Menu) { a.chatMenu(m, ch) })
	if tab.Clicked() {
		a.activate(ch.ID)
	}
	if r := tab.Bounds(); r.W > 0 {
		a.nextRects = append(a.nextRects, tabRect{ID: ch.ID, Rect: r})
	}
	// 标签可以左右拖着排序：拖的时候跟手，松手落到指针下面那个位置。
	// 位移取自指针在标签里的位置（Dragged 报的是一帧的位移，横着会被
	// 滚动容器吃掉，一直是 0），所以记下按下时的位置做参照。
	// Dragged 的一帧位移在横向会被滚动容器吃掉（一直是 0），但它还有一个
	// 必要作用：把标签标成可拖，否则按下后指针会被“转交”给下面的元素，
	// 松手就不算这次拖拽了。真正的位移改用 PointerPosition 算。
	_, _, dragging := tab.Dragged()
	_, _, ended := tab.DragEnded()
	switch {
	case ended && a.dragTab == ch.ID:
		a.dropTab(c, tab)
	case dragging:
		// 不挪动标签本身：位置一动，落点的坐标就跟着漂。拖动时把它压暗，
		// 松手再落到指针下面那一格。
		x, _, _ := tab.PointerPosition()
		if a.dragTab != ch.ID {
			a.dragTab, a.dragTabFrom = ch.ID, x
		}
		if x-a.dragTabFrom != 0 {
			tab.Opacity(0.55)
			tab.Cursor(ui.CursorGrabbing)
		}
	case a.dragTab == ch.ID:
		a.dragTab = ""
	}
	tab.Children(func() {
		a.avatar(c, ch, 16)
		name := ui.Text(c, shorten(ch.Name, 16)).FontSize(12)
		if active {
			name.Bold()
		}
		close := ui.Row(c).Size(17, 17).Radius(4).AlignItems(ui.Center).Justify(ui.Center)
		close.Label("关闭 " + ch.Name)
		close.Tooltip("关闭 " + ch.Name)
		if close.Hovered() {
			close.Background(t.SurfacePressed)
		}
		if close.Clicked() {
			a.closeChat(ch.ID)
		}
		close.Children(func() {
			ui.Icon(c, closeIcon).Size(11, 11).TextColor(t.TextMuted)
		})
	})
}

// toolbar builds the row of a page's own controls: going back and forward
// in it, reloading it, its address, and the chat's buttons.
func (a *app) toolbar(c *ui.Context) {
	t := c.Theme()
	ch := a.store.Find(a.store.Active)
	ui.Row(c).Height(40).Padding(4, 10).Gap(4).AlignItems(ui.Center).
		Background(t.Background).Border(0, t.Border).Children(func() {
		if ch != nil {
			var page *mygo.Page
			if v := a.viewOf(ch); v != nil {
				page = v.Page()
			}
			pageButton(c, "后退", backIcon, page != nil && page.CanGoBack(), func() {
				page.GoBack()
			})
			pageButton(c, "前进", forwardIcon, page != nil && page.CanGoForward(), func() {
				page.GoForward()
			})
			pageButton(c, "重新载入", reloadIcon, page != nil, func() {
				page.Reload()
			})
		}
		// The address bar, as a browser's: the page's own address, which a
		// click copies. The name of the chat is on its tab above.
		address := "chatBox"
		if ch != nil {
			address = ch.URL
			if at := a.pageURL[ch.ID]; at != "" {
				address = at
			}
		}
		// 地址栏只显示当前地址；点进去就能改，回车前往（和浏览器一样）。
		// 编辑中不跟着页面跳转改内容，免得打字打到一半被覆盖。
		if !a.addrFocused {
			a.addrText = address
		}
		// 地址栏只有一圈框线，没有底色：文字直接落在工具栏的白上，
		// 和 Cherry Studio 的一样。指针停上去才浮一层极浅的灰。
		field := ui.TextInputBase(c, &a.addrText).Grow(1).MinWidth(0).Height(28).
			Radius(t.Radius+2).Border(1, t.Border).Padding(2, 10)
		field.Label("地址")
		field.Tooltip("输入网址后回车前往")
		switch {
		case field.Focused():
			field.Background(t.Background)
			field.Border(2, t.Accent)
		case field.Hovered():
			field.Background(t.Surface)
		default:
			field.Background(t.Background)
		}
		if field.Submitted() && ch != nil {
			a.navigate(ch, a.addrText)
		}
		a.addrFocused = field.Focused()
		if ch != nil && ch.Proxy != "" {
			// 走代理或直连的聊天挂一个图标，具体是什么写在 tooltip 里：
			// 工具栏上不再用文字占地方。
			mark := ui.Row(c).Size(24, 24).Radius(t.Radius + 2).
				AlignItems(ui.Center).Justify(ui.Center)
			mark.Label(describeProxyFully(ch.Proxy))
			mark.Tooltip(describeProxyFully(ch.Proxy) + "（点击修改）")
			if mark.Hovered() {
				mark.Background(t.Surface)
			}
			if mark.Clicked() {
				a.openEditor(ch.ID, tabProxy)
			}
			mark.Children(func() {
				ui.Icon(c, proxyIcon).Size(16, 16).TextColor(t.TextMuted)
			})
		}
		if ch != nil {
			// 注入 CSS / JS 在编辑器的「CSS」「JS」页签里，菜单里不再单列一项。
			a.moreButton(c, ch)
		} else {
			if ui.Button(c, "新增聊天").Clicked() {
				a.library = true
			}
		}
	})
}

// content builds the page of the chat open, or what shows in its place.
func (a *app) content(c *ui.Context) {
	t := c.Theme()
	ch := a.store.Find(a.store.Active)
	if ch == nil {
		ui.Column(c).Grow(1).Center().Gap(14).Children(func() {
			ui.Text(c, "chatBox").FontSize(28).Bold()
			ui.Text(c, "把常用的网页版 AI 聊天聚合在一个窗口里").TextColor(t.TextMuted)
			if ui.PrimaryButton(c, "选择一个聊天").Clicked() {
				a.library = true
			}
		})
		return
	}
	v := a.viewOf(ch)
	if v == nil {
		ui.Column(c).Grow(1).Center().Gap(10).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "正在打开…").TextColor(t.TextMuted)
		})
		return
	}
	ui.WebView(c, v).Grow(1)
}

// viewOf returns the chat's web view, nil before the app opens it.
func (a *app) viewOf(ch *Chat) *mygo.WebView {
	if ch == nil {
		return nil
	}
	if v, ok := a.views[ch.ID]; ok && !v.IsDestroyed() {
		return v
	}
	return nil
}

// activate shows a chat: it joins the dock, and the app opens its page.
func (a *app) activate(id string) {
	ch := a.store.Find(id)
	if ch == nil {
		return
	}
	a.store.SetActive(id)
	if err := a.store.Save(); err != nil {
		log.Printf("myChatBox: 保存聊天失败：%v", err)
	}
	a.syncTitle()
	// The view is made between frames: a web view is not made while the
	// window builds one.
	if a.win != nil {
		a.win.Update(func() { a.openView(ch) })
		a.win.Invalidate()
	}
}

// openView creates the chat's web view, once.
func (a *app) openView(ch *Chat) {
	if a.win == nil || a.viewOf(ch) != nil {
		return
	}
	v, err := a.win.NewWebView(mygo.WebViewOptions{
		URL: ch.URL,
		Page: mygo.PageOptions{
			PreloadScript:    preloadScript(ch),
			PreloadAllFrames: ch.AllFrames,
			UserAgent:        strings.TrimSpace(ch.UserAgent),
			ZoomFactor:       zoomOf(ch),
			Session:          a.sessionOf(ch),
			// 打包后的 app 里 mygo 默认关掉网页检查器（它按 production 判断），
			// 但这是给人自己用的聚合浏览器，右键「开发者工具」得能开。
			DevTools: mygo.DevToolsEnabled,
		},
		BackgroundColor: "light-dark(#ffffff, #1e1e1e)",
	})
	if err != nil {
		a.toast("打开 " + ch.Name + " 失败：" + err.Error())
		return
	}
	a.views[ch.ID] = v
	v.Page().OnDidNavigate(func(url string) {
		a.pageURL[ch.ID] = url
		a.win.Invalidate()
	})
	// The page's context menu, as a browser's: the web view's own items,
	// with the app's around them.
	v.Page().OnContextMenu(func(e *mygo.ContextMenuEvent) { a.pageMenu(e, ch, v) })
	v.Page().OnDidFinishLoad(func() {
		a.win.Invalidate()
		if debugWanted() {
			log.Printf("debug: loaded %s <%s>", ch.Name, a.pageURL[ch.ID])
		}
		a.autoIcon(ch, v)
	})
	v.Page().OnDidFailLoad(func(e *mygo.LoadError) {
		if e.URL != "" {
			a.toast("打不开 " + ch.Name + "：" + e.Description)
		}
	})
	a.win.Invalidate()
}

// sessionOf returns the chat's own session: one it asked for, to keep its
// website data apart, and one a proxy needs, since the system takes
// proxies per store. It is nil for a chat that shares the app's default
// one.
func (a *app) sessionOf(ch *Chat) *mygo.Session {
	if !ch.Isolated && strings.TrimSpace(ch.Proxy) == "" {
		return nil
	}
	s, ok := a.sessions[ch.ID]
	if !ok {
		s = mygo.NewSession("chat-" + ch.ID)
		a.sessions[ch.ID] = s
	}
	// Applied every time the chat opens, so that a proxy the user changed,
	// or took away, reaches the pages from their next request on.
	if err := s.SetProxy(strings.TrimSpace(ch.Proxy), ch.ProxyBypass...); err != nil {
		a.toast(ch.Name + " 的代理没生效：" + err.Error())
		// A chat that only wanted a proxy keeps sharing the app's
		// session, and its sign-ins, where the system has none for it.
		if !ch.Isolated {
			return nil
		}
	}
	return s
}

// closeChat takes a chat out of the dock, and closes its page.
func (a *app) closeChat(id string) {
	if v, ok := a.views[id]; ok {
		v.Destroy()
		delete(a.views, id)
		delete(a.pageURL, id)
	}
	a.store.Close(id)
	if err := a.store.Save(); err != nil {
		log.Printf("myChatBox: 保存聊天失败：%v", err)
	}
	a.syncTitle()
	// The chat that takes its place opens too, so the window never waits
	// for one nobody asked for.
	if next := a.store.Active; next != "" && a.win != nil {
		a.activate(next)
		return
	}
	if a.win != nil {
		a.win.Invalidate()
	}
}

// deleteChat deletes a chat from the library, once the user confirms. The
// dialog waits for a frame of its own: a menu reports a choice while the
// window builds one, and a system dialog is not opened there.
func (a *app) deleteChat(id string) {
	if a.win == nil {
		return
	}
	a.win.Update(func() { a.confirmDelete(id) })
}

// confirmDelete asks about deleting a chat, and deletes it.
func (a *app) confirmDelete(id string) {
	ch := a.store.Find(id)
	if ch == nil {
		return
	}
	res, err := mygo.Dialog.Message(mygo.MessageOptions{
		Parent:       a.win,
		Type:         mygo.MessageWarning,
		Title:        "删除聊天",
		Message:      "从聊天列表删除「" + ch.Name + "」？",
		Detail:       "它的图标和自定义的 CSS、JS 会一起删除。",
		Buttons:      []string{"删除", "取消"},
		CancelButton: 1,
	})
	if err != nil || res.Button != 0 {
		return
	}
	a.closeChat(id)
	a.store.Remove(id)
	a.icons.drop(ch.Icon)
	if err := a.store.Save(); err != nil {
		log.Printf("myChatBox: 保存聊天失败：%v", err)
	}
	if a.win != nil {
		a.win.Invalidate()
	}
}

// chatMenu builds the menu of a chat, on its dock item and its toolbar.
type chatAction struct {
	label string
	run   func()
	// disabled grays the item out: “添加到 Dock 栏” of a chat already there.
	disabled bool
}

// chatActions are the items of a chat's menu, which its dock icon, its tab
// and the toolbar's ⋯ all show.
func (a *app) chatActions(ch *Chat) []chatAction {
	return []chatAction{
		{label: "编辑聊天…", run: func() { a.openEditor(ch.ID, tabBasic) }},
		// 固定到 Dock 栏：固定的一直显示，点一次换一种状态。
		a.pinAction(ch),
		{},
		{label: "重新载入", run: func() { a.reloadChat(ch) }},
		{label: "开发者工具", run: func() { a.devTools(ch) }},
		{label: "在浏览器中打开", run: func() {
			if err := mygo.Shell.OpenExternal(ch.URL); err != nil {
				a.toast("打不开浏览器：" + err.Error())
			}
		}},
		{label: "清空缓存…", run: func() { a.clearChatPrompt(ch) }},
		{},
		{label: "关闭标签页", run: func() { a.closeChat(ch.ID) }},
	}
}

// reloadChat reloads a chat's page. The page work waits for the next frame:
// a menu reports its choice while the window builds one, and a web view
// does nothing asked there.
func (a *app) reloadChat(ch *Chat) {
	if a.win == nil {
		return
	}
	a.win.Update(func() {
		v := a.viewOf(ch)
		if v == nil {
			a.toast(ch.Name + " 还没打开")
			return
		}
		v.Page().Reload()
	})
}

// devTools opens the page's inspector, between frames for the same reason.
func (a *app) devTools(ch *Chat) {
	if a.win == nil {
		return
	}
	a.win.Update(func() {
		v := a.viewOf(ch)
		if v == nil {
			a.toast(ch.Name + " 还没打开")
			return
		}
		v.Page().OpenDevTools()
	})
}

// clearChatPrompt asks what to clear of a chat's web data: the cache alone,
// or its sign-ins too, which is what a page stuck in a bad state wants.
func (a *app) clearChatPrompt(ch *Chat) {
	if a.win == nil {
		return
	}
	a.win.Update(func() { a.confirmClear(ch) })
}

// confirmClear is the question clearChatPrompt asks, and doing what it says.
func (a *app) confirmClear(ch *Chat) {
	detail := "只清缓存：网页缓存和服务用不到旧副本，登录状态保留。" +
		"\n连登录一起清：还会清掉 Cookie 和本地存储，需要重新登录。"
	if a.sessions[ch.ID] == nil {
		detail += "\n注意：这个聊天和别的聊天共用网站数据，清登录会把它们的也清掉。"
	}
	res, err := mygo.Dialog.Message(mygo.MessageOptions{
		Parent:       a.win,
		Type:         mygo.MessageWarning,
		Title:        "清空缓存",
		Message:      "清空「" + ch.Name + "」的缓存？",
		Detail:       detail,
		Buttons:      []string{"只清缓存", "连登录一起清", "取消"},
		CancelButton: 2,
	})
	if err != nil || res.Button == 2 || res.Button < 0 {
		return
	}
	a.clearChatData(ch, res.Button == 1)
}

// clearChatData clears a chat's web data, and loads its page afresh so that
// what is gone is really gone.
func (a *app) clearChatData(ch *Chat, withLogin bool) {
	types := mygo.SiteDataCaches | mygo.SiteDataServiceWorkers
	if withLogin {
		types = mygo.SiteDataAll
	}
	// A chat that shares the app's session clears that one.
	s := a.sessions[ch.ID]
	if s == nil {
		s = mygo.DefaultSession()
	}
	if err := s.ClearData(types); err != nil {
		a.toast("清不掉缓存：" + err.Error())
		return
	}
	a.reloadChat(ch)
	if withLogin {
		a.toast("已清空 " + ch.Name + " 的缓存和登录状态")
		return
	}
	a.toast("已清空 " + ch.Name + " 的缓存")
}

// pinAction is the menu item that keeps a chat in the dock: it pins one
// that is loose, and lets a pinned one go.
func (a *app) pinAction(ch *Chat) chatAction {
	if ch.Pinned {
		return chatAction{
			label: "取消固定",
			run:   func() { a.store.Unpin(ch.ID); a.saveStore() },
		}
	}
	return chatAction{
		label: "固定到 Dock 栏",
		run: func() {
			a.store.Pin(ch.ID)
			a.saveStore()
			a.activate(ch.ID)
		},
	}
}

// tabRect is where a tab sat in the frame it was built: the drop looks at
// these to tell which tab the pointer let go over.
type tabRect struct {
	ID string
	ui.Rect
}

// dropTab moves the dragged tab to the place the pointer was let go over,
// and ends the drag.
func (a *app) dropTab(c *ui.Context, tab ui.Element) {
	moved := a.dragTab
	a.dragTab = ""
	if moved == "" {
		return
	}
	// 指针在窗口里的横坐标，用元素自己的局部坐标换算出来。
	localX, _, _ := tab.PointerPosition()
	at := tab.Bounds().X + localX
	to := -1
	for i, r := range a.tabRects {
		if at >= r.X && at <= r.X+r.W {
			to = i
			break
		}
	}
	if to < 0 {
		return
	}
	if a.store.MoveOpenTo(moved, to) {
		a.saveStore()
		if a.win != nil {
			a.win.Invalidate()
		}
	}
}

// sign is 1 for a positive number, -1 for a negative one, 0 for zero: the
// half of a step a drag rounds by.
func sign(v float32) float32 {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// saveStore writes the store, saying what went wrong if it did.
func (a *app) saveStore() {
	if err := a.store.Save(); err != nil {
		a.toast("保存失败：" + err.Error())
	}
}

// chatMenu builds the menu of a chat in the window's own menus, as a
// context menu and a menu button do.
func (a *app) chatMenu(m *ui.Menu, ch *Chat) {
	if ch == nil {
		return
	}
	for _, act := range a.chatActions(ch) {
		if act.label == "" {
			m.Separator()
			continue
		}
		if m.Item(act.label).Disabled(act.disabled).Chosen() {
			act.run()
		}
	}
}

// chatMenuNative is the same menu as one of the system's, which pops up at
// the pointer.
func (a *app) chatMenuNative(ch *Chat) *mygo.Menu {
	var items []*mygo.MenuItem
	for _, act := range a.chatActions(ch) {
		if act.label == "" {
			items = append(items, mygo.Separator())
			continue
		}
		run := act.run
		items = append(items, &mygo.MenuItem{
			Label:    act.label,
			Disabled: act.disabled,
			Click:    func(*mygo.MenuItem, *mygo.Window) { run() },
		})
	}
	return mygo.NewMenu(items)
}

// navigate loads what was typed in the address bar into the chat's page: a
// URL, with or without its scheme, or words to search for.
func (a *app) navigate(ch *Chat, typed string) {
	v := a.viewOf(ch)
	if v == nil {
		return
	}
	text := strings.TrimSpace(typed)
	if text == "" {
		return
	}
	target := a.addressTarget(text)
	if err := v.Page().LoadURL(target); err != nil {
		a.toast("打不开这个地址：" + err.Error())
		return
	}
	a.pageURL[ch.ID] = target
	a.addrText, a.addrFocused = target, false
}

// addressTarget turns what was typed in the address bar into what the page
// loads: a URL, with a scheme added when it came without one, or a search
// for words that are no URL.
func (a *app) addressTarget(text string) string {
	if u, err := parseURL(text); err == nil {
		return u.String()
	}
	if !strings.Contains(text, ".") {
		return a.searchURL(text)
	}
	return text
}

// moreButton is the toolbar's ⋯: three dots with no frame of their own,
// which pop up the chat's menu at the pointer.
func (a *app) moreButton(c *ui.Context, ch *Chat) {
	t := c.Theme()
	b := ui.Row(c).Width(30).Height(26).Radius(t.Radius + 4).
		AlignItems(ui.Center).Justify(ui.Center)
	b.Label("更多")
	b.Tooltip("更多")
	if b.Hovered() {
		b.Background(t.Surface)
	}
	if b.Clicked() {
		// The menu opens between frames, as the system's menus do.
		a.win.Update(func() { a.chatMenuNative(ch).Popup(a.win) })
	}
	b.Children(func() {
		ui.Icon(c, moreIcon).Size(17, 17).TextColor(t.Text)
	})
}

// openEditor opens the editor on a chat, or on a new one.
func (a *app) openEditor(id string, tab int) {
	a.editingID = id
	a.editorTab = tab
	d := &a.draft
	*d = draft{Zoom: "100%", ID: newID()}
	d.ProxyMode = proxyModeOf("")
	if ch := a.store.Find(id); ch != nil {
		d.ID = ch.ID
		d.Name, d.URL = ch.Name, ch.URL
		d.CSS, d.JS = ch.CSS, ch.JS
		d.UserAgent = ch.UserAgent
		d.Zoom = zoomLabel(ch.Zoom)
		d.Isolated, d.AllFrames = ch.Isolated, ch.AllFrames
		d.IconName, d.IconPic = ch.Icon, a.icons.get(ch.Icon)
		d.ProxyMode = proxyModeOf(ch.Proxy)
		if d.ProxyMode == proxyModes[2] {
			d.ProxyURL = ch.Proxy
		}
		d.Bypass = strings.Join(ch.ProxyBypass, "\n")
	}
	a.detectSeq++
	a.library = false
	a.editor = true
}

// saveDraft saves the chat the editor edits.
func (a *app) saveDraft() {
	d := &a.draft
	u, err := parseURL(d.URL)
	if err != nil {
		d.Error, a.editorTab = err.Error(), tabBasic
		return
	}
	proxy := proxySpecOf(d.ProxyMode, d.ProxyURL)
	if err := validateProxy(proxy); err != nil {
		d.Error, a.editorTab = err.Error(), tabProxy
		return
	}
	bypass := parseBypass(d.Bypass)
	d.Error = ""
	name := strings.TrimSpace(d.Name)
	if name == "" {
		name = hostOf(u.String())
	}
	if d.ID == "" {
		d.ID = newID()
	}
	ch := a.store.Find(a.editingID)
	if ch == nil {
		ch = &Chat{ID: d.ID}
		a.store.Add(ch)
		ch.Builtin = false
	}
	old := *ch
	ch.Name, ch.URL = name, u.String()
	ch.CSS, ch.JS = d.CSS, d.JS
	ch.UserAgent = strings.TrimSpace(d.UserAgent)
	ch.Zoom = zoomValue(d.Zoom)
	ch.Isolated, ch.AllFrames = d.Isolated, d.AllFrames
	ch.Proxy, ch.ProxyBypass = proxy, bypass
	ch.Icon = d.IconName
	if ch.Icon != old.Icon && old.Icon != "" {
		a.icons.drop(old.Icon)
	}

	a.store.SetActive(ch.ID)
	if err := a.store.Save(); err != nil {
		log.Printf("myChatBox: 保存聊天失败：%v", err)
	}
	a.syncTitle()
	a.editor = false
	a.toast("已保存「" + ch.Name + "」")

	// What changes how the page loads makes a new web view; CSS and JS
	// reach the page open as it is.
	rebuild := old.URL != ch.URL || old.UserAgent != ch.UserAgent ||
		old.Isolated != ch.Isolated || old.AllFrames != ch.AllFrames ||
		old.Proxy != ch.Proxy || !slices.Equal(old.ProxyBypass, ch.ProxyBypass)
	switch {
	case rebuild:
		if v, ok := a.views[ch.ID]; ok {
			v.Destroy()
			delete(a.views, ch.ID)
		}
		a.activate(ch.ID)
	case a.viewOf(ch) != nil:
		if old.Zoom != ch.Zoom {
			if v := a.viewOf(ch); v != nil {
				v.Page().SetZoomFactor(zoomOf(ch))
			}
		}
		a.applyInject(ch, true)
		if a.win != nil {
			a.win.Invalidate()
		}
	default:
		a.activate(ch.ID)
	}
}

// applyInject applies a chat's CSS, and runs its JS, on its page as the
// page is now.
func (a *app) applyInject(ch *Chat, runJS bool) {
	v := a.viewOf(ch)
	if v == nil {
		return
	}
	script := applyScript(ch, runJS)
	go func() {
		if _, err := v.Page().Eval(script); err != nil {
			log.Printf("myChatBox: 注入失败：%v", err)
		}
	}()
}

// autoIcon reads the icon a chat's page links, once, and keeps it in the
// dock while the chat has none.
func (a *app) autoIcon(ch *Chat, v *mygo.WebView) {
	if ch.Icon != "" {
		return
	}
	a.iconMu.Lock()
	if a.iconTried[ch.ID] {
		a.iconMu.Unlock()
		return
	}
	a.iconTried[ch.ID] = true
	a.iconMu.Unlock()

	pageURL := v.Page().URL()
	if pageURL == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		data := iconFromPage(ctx, v, pageURL)
		if len(data) == 0 {
			return
		}
		a.win.Update(func() {
			current := a.store.Find(ch.ID)
			if current == nil || current.Icon != "" {
				return
			}
			name, _, err := a.icons.put(current.ID, data)
			if err != nil {
				return
			}
			current.Icon = name
			if err := a.store.Save(); err != nil {
				log.Printf("myChatBox: 保存聊天失败：%v", err)
			}
			a.win.Invalidate()
		})
	}()
}

// iconFromPage returns the icon of a page the app has open: the first of
// its links the app can read, or the favicon at the root of the site.
func iconFromPage(ctx context.Context, v *mygo.WebView, pageURL string) []byte {
	var hrefs []string
	val, err := v.Page().Eval(`Array.from(document.querySelectorAll('link[rel~="icon"], link[rel="apple-touch-icon"], link[rel="mask-icon"]')).map(function (l) { return l.href })`)
	if err == nil {
		if list, ok := val.([]any); ok {
			for _, item := range list {
				if href, ok := item.(string); ok && href != "" {
					hrefs = append(hrefs, href)
				}
			}
		}
	}
	sort.SliceStable(hrefs, func(i, j int) bool { return iconRank(hrefs[i]) > iconRank(hrefs[j]) })
	if u, err := url.Parse(pageURL); err == nil && u.Host != "" {
		hrefs = append(hrefs, u.Scheme+"://"+u.Host+"/favicon.ico")
	}
	for _, href := range hrefs {
		if data, _, err := fetchImage(ctx, href, pageURL); err == nil {
			return data
		}
	}
	return nil
}

// iconRank orders the links of a page: a vector icon first, then a bitmap,
// the favicon of a site last.
func iconRank(href string) int {
	switch {
	case strings.Contains(href, ".svg"):
		return 3
	case strings.Contains(href, ".png"), strings.Contains(href, ".webp"):
		return 2
	default:
		return 1
	}
}

// detectDraft recognizes the name and icon of the URL the editor holds.
func (a *app) detectDraft() {
	d := &a.draft
	if d.Detecting {
		return
	}
	if strings.TrimSpace(d.URL) == "" || d.URL == "https://" {
		d.Error = "先填写网址"
		return
	}
	a.detectSeq++
	seq := a.detectSeq
	d.Detecting, d.Error, d.Message = true, "", ""
	a.win.Invalidate()
	raw := d.URL
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		meta, err := detectMeta(ctx, raw)
		a.win.Update(func() {
			if seq != a.detectSeq || !a.editor {
				return
			}
			d.Detecting = false
			if err != nil {
				d.Error = "识别失败：" + err.Error()
				return
			}
			if d.ID == "" {
				d.ID = newID()
			}
			if strings.TrimSpace(d.Name) == "" && meta.Title != "" {
				d.Name = meta.Title
			}
			msg := "已识别"
			if meta.Title != "" {
				msg += "：" + meta.Title
			}
			if len(meta.IconData) > 0 {
				if name, pic, err := a.icons.put(d.ID, meta.IconData); err == nil {
					d.IconName, d.IconPic = name, pic
					msg += "，图标已更新"
				} else {
					msg += "，图标读取失败"
				}
			} else {
				msg += "，没找到图标"
			}
			d.Message = msg
		})
	}()
}

// uploadIcon sets the chat's icon from a file the user chooses. The file
// dialog opens between frames, as deleteChat's does.
func (a *app) uploadIcon() {
	if a.win == nil {
		return
	}
	a.win.Update(a.pickIcon)
}

// pickIcon shows the file dialog and keeps the icon chosen.
func (a *app) pickIcon() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:            a.win,
		Title:             "选择图标",
		ButtonLabel:       "使用",
		Filters:           []mygo.FileFilter{{Name: "图片", Extensions: []string{"png", "jpg", "jpeg", "gif", "webp", "svg", "ico"}}},
		CreateDirectories: true,
	})
	if err != nil {
		a.toast("打开文件失败：" + err.Error())
		return
	}
	if len(paths) == 0 {
		return
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		a.toast("读取图片失败：" + err.Error())
		return
	}
	a.win.Update(func() {
		d := &a.draft
		if d.ID == "" {
			d.ID = newID()
		}
		name, pic, err := a.icons.put(d.ID, data)
		if err != nil {
			d.Error = "这个图片不能用：" + err.Error()
			return
		}
		if d.IconName != "" {
			a.icons.drop(d.IconName)
		}
		d.IconName, d.IconPic = name, pic
		d.Message, d.Error = "图标已更新", ""
	})
}

// clearIcon removes the chat's icon.
func (a *app) clearIcon() {
	d := &a.draft
	if d.IconName != "" {
		a.icons.drop(d.IconName)
	}
	d.IconName, d.IconPic = "", nil
}

// toast shows a message over the window.
func (a *app) toast(msg string) {
	if a.win == nil {
		return
	}
	a.win.Update(func() { a.pendingToast = msg })
	a.win.Invalidate()
}

// syncTitle names the chat open in the window's title bar.
func (a *app) syncTitle() {
	if a.win == nil {
		return
	}
	title := "chatBox"
	if ch := a.store.Find(a.store.Active); ch != nil {
		title = ch.Name + " — chatBox"
	}
	a.win.SetTitle(title)
}

// zoomOf returns the scale of a chat's pages, 1 when it has none.
func zoomOf(ch *Chat) float64 {
	if ch == nil || ch.Zoom <= 0 {
		return 1
	}
	return ch.Zoom
}

// zoomValue reads a zoom label, as "125%".
func zoomValue(label string) float64 {
	var n float64
	if _, err := fmt.Sscanf(strings.TrimSuffix(label, "%"), "%g", &n); err != nil || n <= 0 {
		return 1
	}
	return n / 100
}

// zoomLabel names a zoom scale, as "125%".
func zoomLabel(zoom float64) string {
	if zoom <= 0 {
		zoom = 1
	}
	return strconv.FormatFloat(zoom*100, 'f', -1, 64) + "%"
}
