# chatBox

English | [简体中文](README.zh-CN.md)

[![build](https://github.com/weigram/chatBox/actions/workflows/build.yml/badge.svg)](https://github.com/weigram/chatBox/actions/workflows/build.yml)

Every web chat you use, in one window. A dock on the left holds each chat you
have open — click one to switch, and its page keeps running in the background
instead of reloading. Each chat can carry its own CSS and JavaScript too.

Built with [mygo](https://github.com/egoist/mygo): pure Go, no cgo, no frontend
build. Pages run in the system's own web view (WKWebView on macOS) and the
window is native UI drawn by mygo. One binary, a dozen megabytes.

![chatBox: the dock on the left, the chats as tabs in the title bar, and ChatGPT's page](docs/screenshot.png)

The dock on the left holds every chat you have open — they are the tabs in the
title bar too — and the page fills the rest of the window.

## Features

- **Built-in chats**: ChatGPT, Claude, Gemini, DeepSeek, Doubao, Kimi, Qwen,
  Zhipu Qingyan, Ernie, Tencent Yuanbao, Grok, Mistral, Perplexity, Copilot.
  They are ordinary chats: rename them, change their URL, delete them, or
  restore them all with one click.
- **Add your own**: a name, a URL and an icon. Icons may be PNG, JPEG, GIF,
  WebP, BMP, SVG or ICO.
- **Recognised from the URL**: 识别 in the editor reads the page's `<title>`
  and its `<link rel="icon">` (including `apple-touch-icon`, falling back to
  `/favicon.ico`). A chat with no icon also gets one read from the page the
  first time it loads, and cached.
- **The dock**: **pinned chats sit at the top**, a thin line, and **below it
  the chats that are open now**. Closing a pinned chat's tab leaves it there;
  click it to bring it back. **Pinned icons can be dragged up and down to
  reorder them** (they follow the pointer and drop into place, and the order
  is saved in `chats.json`); the group below is in the order they were opened
  and does not move. The rail is **48px** wide, as Cherry Studio's is: circles
  32, icons 22, 4 from each edge. Every icon sits on a circle of the same size;
  the chat the window shows wears **plain white with a little shadow**, the
  others a translucent white (choosing a chat never changes its size), and the
  one under the pointer is whiter still. The whole row is a target. Right-click
  an icon to edit, reload, open the inspector or close. A chat with no icon
  shows its initial in a circle. The bottom holds a single icon-only gear
  (settings), with no background until the pointer is on it; new chats come
  from the tab strip's ＋ and `⌘N`.
- **Chats stay alive**: switching away only hides a page. It keeps running in
  the background and comes back as it was; closing its tab is what frees it.
  Cookies and sign-ins are shared by default and kept across launches.
- **Injected CSS and JS**: one pair per chat, kept in the config and applied as
  the page loads. Saving applies them to the open page at once, without a
  reload.
- **A proxy per chat**: follow the system, connect directly, or a proxy of its
  own over http, https or socks5 (with a user and password in the URL, and
  hosts to bypass). A chat with a proxy sends **all** of its page's requests
  through it — third-party CDNs, sign-ins, APIs — and no other chat is
  affected.
  the dock's colour too) and the chosen tab is a white pill on it with a soft
  shadow. Dock circles are all the same size, the chosen one the whitest. The
  toolbar's buttons have no background until the pointer rests on them. The
  dark theme swaps these around on its own.
- **An editable address bar**: the URL field has a 1px outline and no fill,
  and **clicking it lets you type**. Enter goes there: `https://` is added when
  missing, and words that are no URL are searched for. The field turns accent
  coloured while focused, and a page that redirects never overwrites what you
  are typing.
- **The address row is hidden by default**: back, forward, reload, the URL, the
  proxy icon and the ⋯ menu stay away unless you ask for them in 设置 →
  显示地址栏, leaving only tabs and the page itself. Reload, the inspector and
  "open in browser" are still in a tab's context menu.
- **Tabs can be dragged**: press a tab and drag it sideways; it dims while you
  drag and lands where the pointer is. That order is the order of the dock's
  second group, and both are saved in `chats.json`.
- **Tabs live in the title bar**: the window uses a hidden title bar, so the
  tab strip is the window's top row, with the traffic lights over its left edge
  (on the dock's colour, so the column reads as one). That saves a row over a
  browser-like layout, and the empty part of the row drags the window. Each
  chat is a tab with its icon, name and ×; the ＋ at the end opens another.
- **A global shortcut**: press `⌘⇧M` in any application (after turning 启用 on
  in 设置) to show or hide the window. The keys are set in 设置.
- **Always on top**: the pin at the **far right of the tab strip** (the window's
  top-right corner). Lit means it is on; it takes effect at once and is
  remembered.
- **Also**: back and forward, a custom User-Agent, page zoom, an isolated
  session (this chat's cookies and cache apart from the others'), injection
  into iframes, and a remembered window position.

## Running

**Go 1.27.3 or newer** is required: `go.mod` says so, and older toolchains
cannot build the module at all.

```sh
make run
```

`go run .` works too. The first run writes the built-in chats and puts four in
the dock (ChatGPT, Claude, Gemini, DeepSeek); only the one you click loads.

> Network: `proxy.golang.org` is often out of reach in China, so the `Makefile`
> defaults to `GOPROXY=https://goproxy.cn,direct`. `export GOPROXY=...` for
> another mirror.
>
> Caches: the Go ones by default. In an environment that may only write its own
> directory (a sandbox, a throwaway machine), `make LOCAL=1 run` keeps them in
> `.cache/`, which is git-ignored.

## Using it

| To | Do |
| --- | --- |
| Open a chat | The ＋ at the end of the tab strip, then pick one from 聊天列表; or `⌘N` for a new one |
| Switch chats | Click a dock icon, or a tab |
| Rename, change the URL or icon | The **pencil at the end of a row** in 聊天列表, or right-click a dock icon → 编辑聊天… |
| **Pin to the dock** | Right-click a tab, a dock icon or a list row → 固定到 Dock 栏. A pinned chat **stays at the top of the dock** even with its tab closed; the same item then turns into 取消固定 |
| Recognise a name and icon | Fill in the URL in the editor and press 识别 |
| Go to another page | Type a URL in the address bar and press Enter (`https://` is added; words are searched for) |
| Injected CSS / JS | 编辑聊天… → the CSS 注入 and JS 注入 tabs |
| Close a tab | The × on the tab, or right-click a dock icon → 关闭标签页 |
| Reload, or open the inspector | Right-click a tab or dock icon; both are in the menu, and the inspector **works in the packaged .app** too |
| Delete a chat for good | Right-click its row in 聊天列表 → 从列表中删除, which asks first |
| Clear the cache, or sign out | Right-click a tab or dock icon → 清空缓存…: the cache alone, or cookies and site storage too |
| Find other chats | The ＋ at the end of the tab strip: 聊天列表 holds every built-in chat and your own |
| Show or hide the window | `⌘⇧M` in any application (the default; 设置 changes it) |
| Keep the window above others | The pin at the far right of the tab strip |
| Change the shortcut, see the data directory | The gear at the bottom of the dock |
| Give a chat a proxy | Right-click a dock icon → 编辑聊天… → the 代理 tab |
| Right-click a page | The system's menu (copy, paste, lookup, translate, spelling, services, inspect) plus the browser's own items, below |

The window's own shortcuts work while the native UI has the focus (keys go to
the page when it does): `⌘N` opens a new chat and `⌘L` the chat list. The global
shortcut is not bound by that.

## A proxy per chat

The 代理 tab in the editor offers three choices:

| Choice | Meaning |
| --- | --- |
| 跟随系统 | The default: the system's proxy decides, and the app keeps out of it |
| 直连 | Whatever the system says, this chat connects directly (common for an intranet chat) |
| 自定义 | `http://127.0.0.1:7890`, `https://…` or `socks5://[user:pass@]host:port`; hosts to reach directly are listed below it, their subdomains included |

Things worth knowing:

- **The unit is the chat, not the domain**: every request a chat's pages make —
  the site, its CDNs, sign-ins, APIs — goes through that one proxy, while other
  chats do as their own settings say.
- **A chat with a proxy (or 直连) gets a session of its own**: its cookies,
  sign-ins and cache apart from the others'. That is the system's rule —
  proxies are taken per website data store, not per window — and it is what
  keeps ChatGPT's proxy from carrying Doubao's requests with it. The flip side:
  **turning a proxy on for a chat that shared the app's session signs it out
  once** (the old cookies stay in the shared session), and setting it back to
  跟随系统 brings that sign-in back.
- **macOS 14 or later**: this goes through
  `WKWebsiteDataStore.proxyConfigurations`. Older systems say the proxy could
  not be applied, and the chat keeps working on the shared session and the
  system's proxy.
- **The page reloads** after the change: a proxy reaches the page's next
  request, and a reload is the cleanest way to get there.
- A chat with a proxy or 直连 shows a **proxy icon** (a bolt in a circle) right
  of the address bar; hovering says which proxy, and clicking opens its proxy
  settings.

To check that it really goes through: the repository holds a local CONNECT
proxy that only logs.

```sh
go run ./tools/testproxy 127.0.0.1:18888     # in another terminal
# Then set a chat's proxy to http://127.0.0.1:18888 and open it: that side
# prints lines such as CONNECT example.com:443, and that is the chat going
# through it.
```

### Why mygo is bundled

WKWebView on macOS has **no** per-web-view proxy switch, and it **ignores**
`HTTP_PROXY` / `HTTPS_PROXY` (tried): a system web view takes proxies per
website data store, and mygo had neither that field nor a way to reach the data
store. So the project carries a patched copy of mygo (`third_party/mygo`, which
`go.mod` points at with `replace`): `Session.SetProxy` was added, and the Darwin
backend implements it with Network.framework and
`WKWebsiteDataStore.proxyConfigurations`. The change is small; what it does and
how to carry it to a new mygo are in
[third_party/mygo/PATCH.md](third_party/mygo/PATCH.md).

## The page's context menu

A right-click in a page is WebKit's own menu, and the app **adds** the items a
browser has rather than replacing it (replacing it would lose translate,
lookup, spelling and services):

| Where | Items |
| --- | --- |
| Top | Back, forward, reload (skipped where WebKit has them) |
| On selected text | Copy, **copy a link to the selection**, search the web for "…" |
| On a link | Copy the link, open it in the default browser |
| Always | Copy the page's address, open the page in the browser, print…, inspect element |

- **Copy a link to the selection** is what Safari does: it gives you a URL with
  a text fragment (`#:~:text=…`), and a browser opens it scrolled to and
  highlighting that text.
- **Search the web** uses the engine in 设置 → 搜索用网址, where `%s` is the
  query; the default is `https://cn.bing.com/search?q=%s`.
- To see the items of a menu as they are, log them:

  ```sh
  CHATBOX_DEBUG_MENU=1 CHATBOX_DEBUG_LOG=/tmp/menu.log make run
  # Right-click in the page, and /tmp/menu.log gets
  # debug: page menu of …: [ … ]
  ```

- This follows WebKit's menu, so it was checked on macOS. mygo has the same
  interface on Windows and Linux, but it was not checked there.

## The global shortcut, and always on top

- **Closing the window is not quitting**: the red light (or `⌘W`) only **puts
  the window away**. The app keeps running, so the global shortcut still works,
  and pressing it or clicking the dock icon brings the window back. Quitting is
  `⌘Q`.
- **The default keys** are `CmdOrCtrl+Shift+M` (`⌘⇧M` on macOS), but the
  shortcut is **off by default**: the first run fills the keys in, and 设置 →
  启用 turns it on.
- **Recording keys**: click the box in 设置 and press the keys you want (one of
  ⌘ / ⌥ / ⌃ / ⇧ is required). `Esc` cancels, `⌫` clears. **Changing the keys
  turns 启用 off**, so it is turned on again deliberately.
- **Checked before it is turned on**: turning 启用 on tries the combination
  with the system first. When another application holds it the system refuses,
  and a dialog explains: either leave it off, or 仍然启用 — on, but taking
  effect only once the other application lets the keys go. The window's own
  `⌘N` / `⌘L` are warned about too, so the app does not compete with itself.
- **What one press does**: with the window in front (visible, not minimised and
  the app active) it puts it away; otherwise it brings the window up and
  focuses it.
- **Spelling**: recording makes mygo's accelerator, such as `Super+Shift+M`
  (`Super` is macOS's ⌘); editing `chats.json` by hand may also use
  `CmdOrCtrl+Shift+M` (`Super+Shift+M` once recorded).
- **In the background**: closing the window does not end the process (the
  shortcut would go with it) and does not reload the pages — what you were
  typing and where you had scrolled are still there when it comes back.
- **Always on top**: the pin at the far right of the tab strip (the window's
  top-right corner) turns into a small white circle (22px, a size smaller than
  the toolbar's buttons). It is remembered, so the next launch starts on top.

## Injected CSS and JS

- CSS goes in at document start, before the page's own scripts, so elements can
  be hidden before they flash; JS runs after `DOMContentLoaded`, so it can read
  the DOM.
- Saving applies them to the open page at once and writes them to the page's
  `localStorage`, so reloads and navigations keep them. No web view is rebuilt
  and a conversation in progress is not interrupted.
- 高级选项 → 将 CSS 与 JS 也注入页面里的 iframe turns on `allFrames`, for sites
  that keep their content in an iframe.
- A script that throws only reaches the page's console (`[chatBox] 注入脚本
  出错`); the app is not affected.
- To debug in the page's own tools: right-click → 开发者工具.

## Where the data lives

macOS: `~/Library/Application Support/chatBox/`

```
chats.json        the chats (with their proxies and bypass hosts), the dock's
                  order, the current chat, the shortcut and the pin's state
icons/            chat icons, cached as needed
```

- Somewhere else (a USB stick, the project directory, for backups) with an
  environment variable:

  ```sh
  CHATBOX_HOME=/path/to/dir go run .
  ```

- `chats.json` is plain JSON: read it, edit it, restart the app.
- Cookies and site storage live in the system web view's own directories —
  `~/Library/WebKit/app.mychatbox.desktop/` and
  `~/Library/HTTPStorages/app.mychatbox.desktop.binarycookies` — with a
  directory per session for the chats that have one of their own (a proxy or
  an isolated session).

## Building

```sh
make build     # build/darwin-arm64/chatBox.app
make dmg       # and a disk image
```

The name, bundle id and version are in `mygo.json`; the icon is
`resources/icon.png` (`make icon` redraws it with `tools/mkicon`).

The bundle id is still `app.mychatbox.desktop`, from the app's old name, and
**on purpose**: the system files cookies and site storage under it, so changing
it would sign you out of every chat. Change it if you want a clean identity,
expecting that one sign-out.

Drawing the icon has one rule: the white bubble's tail must stay **clear of the
rounded corners**. Too close, and at dock sizes (32–64px) the corner loses its
ring of colour and the icon looks cut off at the lower left. `tools/mkicon`
measures it: the white must keep at least 5.5% of the tile's edge to itself, or
the tool reports it and writes nothing.

> After changing the icon, macOS may keep showing the old one: `killall Dock`.

## The code

```
main.go      startup: app name, data directory, the window
app.go       app state: open, switch and close chats, save the editor,
             recognise names, the dock and the toolbar
view.go      the dock, the chat list, the editor and the settings
model.go     Chat and Store: chats.json, the built-in chats, pinning
proxy.go     the three proxy modes, checking and describing them
recorder.go  recording the shortcut, showing it, checking it with the system
pagemenus.go the page's context menu: WebKit's items plus a browser's
widgets.go   small widgets and icons: pin, back/forward/reload, ＋, gear, ×,
             the proxy mark, and the icon buttons around them
meta.go      a page's title and icon from its URL (HTTP and x/net/html)
icons.go     icon decoding (PNG inside ICO too), scaling and caching
inject.go    the injection script's template, applying it live
debug.go     the harness that runs the app, captures and reports (below)
third_party/mygo  the bundled mygo, patched for a proxy per session
tools/       mkicon (draws the app icon), testproxy (a logging CONNECT proxy),
             watchbuild (rebuilds on change)
```

## Development

```sh
make test    # the views through ui.NewTester, no window needed
make vet
make watch   # stays running: any .go or asset change runs make build
```

`make watch` is `tools/watchbuild`. It polls file times (no fswatch needed) and
runs `make build` a second after any change to a `.go` file, `mygo.json` or an
image in `resources/`, so `build/darwin-arm64/chatBox.app` is always current.
A change that does not compile only logs the error; the watcher keeps going.

```sh
make watch
# 20:03:19 watching …; a change runs make build
# 20:03:38 built in 660ms
#   ✓ Built chatBox 0.1.0 in 555ms
```

Or build on demand with `make build`, and `make dmg` for a disk image.

The app has a self-check that only runs when an environment variable asks for
it: it opens the first chat in the dock, waits for the page, prints the title
and the injection's result, captures the window to a PNG, and exits.

```sh
CHATBOX_DEBUG_CAPTURE=/tmp/shot.png \
CHATBOX_DEBUG_URL=https://example.com/ \
CHATBOX_DEBUG_INJECT=1 \
go run .
```

`CHATBOX_DEBUG_DIALOG=library|editor|css|js|settings|proxy` captures each
dialog; `CHATBOX_DEBUG_MENU=1` logs the items of a page's context menu (with
`CHATBOX_DEBUG_LOG=<file>` to write to a file, easier to read when driving the
UI); `CHATBOX_DEBUG_TOGGLE=1` also checks the global shortcut's registration,
showing and hiding the window, the pin, and the warning for keys the system
refuses; `CHATBOX_DEBUG_RECORD=Super+Shift+K` records a shortcut in the
settings; `CHATBOX_DEBUG_CLOSE=1` closes the window and reports what became of
it.

The proxy can be checked on its own: `CHATBOX_DEBUG_PROXY=http://127.0.0.1:18888`
sends the first chat through that proxy, and `CHATBOX_DEBUG_SECOND=1` opens a
second chat without one — only the first should show a `CONNECT` in
`tools/testproxy`'s log.

## Known limits, and what could come next

- One window, on the desktop. mygo also runs on Linux and Windows and
  `mygo build -platform` cross-packages, but the sizes and shortcuts were only
  checked on macOS.
- Chats are not grouped, and a chat's position in the chat list is not editable.
- A dock icon has no close button on hover (close from the tab's × or the
  context menu).
- The dock and the tab strip still show the same list (the chats that are
  open), apart from pinning. Cherry Studio's left side is *all* assistants;
  making the dock "every chat" and the strip "the open ones" is easy if wanted.
- Recognising a name and icon depends on the HTML the site returns; a page drawn
  entirely by script, that also refuses non-browser user agents, can fail.
  Names and icons can be typed in and uploaded.

## License

MIT for the bundled mygo, see [third_party/mygo/LICENSE](third_party/mygo/LICENSE).
The app itself has no LICENSE file yet; pick one and add it at the root.
