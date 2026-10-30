package main

import (
	"strings"
	"unicode"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// settingsView builds the settings dialog: the global shortcut that shows
// and hides the window, whether the page row shows, where the app searches
// from, and where it keeps its files.
func (a *app) settingsView(c *ui.Context) {
	if !a.settings {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.settings, func() {
		ui.Column(c).Width(520).Gap(12).Children(func() {
			ui.Text(c, "设置").FontSize(18).Bold()
			ui.Field(c, "全局快捷键", func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					a.shortcutRecorder(c)
					// 打开启用时先问系统能不能拿到这个组合：拿不到就先弹框，
					// 确认之后才真的打开。
					on := a.setShortcutOn
					sw := ui.Switch(c, &on)
					// 给开关一个名字：读屏和测试都按它找控件。
					sw.Label("启用快捷键")
					if sw.Changed() {
						if on {
							a.tryEnable()
						} else {
							a.setShortcutOn, a.recError = false, ""
						}
					}
					ui.Text(c, "启用").TextColor(t.TextMuted)
				})
			})
			ui.Divider(c)
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Switch(c, &a.setToolbar).Label("显示地址栏")
				ui.Text(c, "显示地址栏")
			})
			ui.Divider(c)
			ui.Field(c, "搜索用网址", func() {
				ui.TextInput(c, &a.setSearch).Placeholder(defaultSearchURL).Grow(1)
			})
			ui.Divider(c)
			ui.Field(c, "数据目录", func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, a.dataDir).FontSize(11).TextColor(t.TextMuted).Grow(1)
					if ui.Button(c, "打开").Clicked() {
						if err := mygo.Shell.OpenPath(a.dataDir); err != nil {
							a.toast("打不开数据目录：" + err.Error())
						}
					}
				})
			})
			switch {
			case a.settingsErr:
				ui.Text(c, a.settingsMsg).FontSize(12).TextColor(t.Danger)
			case a.settingsMsg != "":
				ui.Text(c, a.settingsMsg).FontSize(12).TextColor(t.TextMuted)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					a.settings = false
					a.recording, a.confirmShortcut = false, false
					// The shortcut claimed while the dialog was open is
					// the stored one again.
					_ = a.registerShortcut()
				}
				if ui.PrimaryButton(c, "保存").Clicked() {
					a.saveSettings()
				}
			})
		})
	})
}

// shortcutRecorder is the box that records the global shortcut: a click
// starts listening, the keys that follow become the accelerator, and Esc
// puts the recording away.
func (a *app) shortcutRecorder(c *ui.Context) {
	t := c.Theme()
	box := ui.Row(c).Width(230).Height(30).Radius(t.Radius+2).
		AlignItems(ui.Center).Padding(2, 10).Gap(8)
	box.Label("快捷键")
	if a.recording {
		box.Border(2, t.Accent)
		box.Background(t.Background)
	} else {
		box.Border(1, t.Border)
		box.Background(t.Background)
	}
	if box.Clicked() && !a.recording {
		a.recording, a.recFocused, a.recError = true, false, ""
	}
	if a.recording {
		// The recorder takes the keys as they come, so Tab and the arrows
		// do not move the focus away while it listens. It takes the focus
		// once, as it starts: asking for it on every frame would keep the
		// view asking for frames.
		if !a.recFocused {
			a.recFocused = true
			box.Focus()
		}
		box.HandleInput(a.recordKey)
	}
	box.Children(func() {
		text := displayAccel(a.setShortcut)
		if a.recording {
			text = "按下组合键…"
		}
		ui.Text(c, text).FontSize(12).TextColor(t.Text)
		ui.Row(c).Grow(1)
		if !a.recording {
			ui.Text(c, "录制").FontSize(11).TextColor(t.Accent)
		}
	})
}

// confirmShortcutView asks about an accelerator the system refused: another
// application holds it, most of the time, and the user may still want it
// for when that application quits.
func (a *app) confirmShortcutView(c *ui.Context) {
	if !a.confirmShortcut {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.confirmShortcut, func() {
		ui.Column(c).Width(430).Gap(10).Children(func() {
			title := "快捷键可能被占用"
			if a.confirmForEnable {
				title = "这个快捷键用不了"
			}
			ui.Text(c, title).FontSize(16).Bold()
			ui.Text(c, displayAccel(a.recPending)+"（"+a.recPending+"）").FontSize(13)
			body := conflictReason(a.recError) + "。仍然保存的话，等它空出来就会生效；" +
				"在这之前按它不会有反应。"
			if a.confirmForEnable {
				body = conflictReason(a.recError) + "。现在打开也不会生效；" +
					"可以先关掉它，或者等那个应用放开这个组合再来开。"
			}
			ui.Text(c, body).FontSize(12).TextColor(t.TextMuted)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if a.confirmForEnable {
					if ui.Button(c, "先不开").Clicked() {
						a.confirmShortcut, a.confirmForEnable, a.recPending = false, false, ""
						a.setShortcutOn = false
					}
					if ui.PrimaryButton(c, "仍然启用").Clicked() {
						a.confirmForEnable = false
						a.enableRecorded()
					}
					return
				}
				if ui.Button(c, "换一个").Clicked() {
					a.confirmShortcut, a.recPending = false, ""
					a.recording = true
				}
				if ui.PrimaryButton(c, "仍然保存").Clicked() {
					a.confirmShortcut = false
					a.keepRecorded()
				}
			})
		})
	})
}

// libraryView builds the list of every chat, from which one opens in the
// dock.
func (a *app) libraryView(c *ui.Context) {
	if !a.library {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.library, func() {
		ui.Column(c).Width(520).Gap(10).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, "聊天列表").FontSize(17).Bold()
					ui.Text(c, "选择后打开到左侧 Dock 栏").FontSize(12).TextColor(t.TextMuted)
				})
				if ui.PrimaryButton(c, "＋ 新建聊天").Clicked() {
					a.openEditor("", tabBasic)
				}
			})
			ui.Scroll(c).Height(400).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					for _, ch := range a.store.All() {
						a.libraryRow(c, ch)
					}
				})
			})
			ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				if ui.Button(c, "恢复内置聊天").Tooltip(
					"把内置的 14 个网站加回列表：已经有的（按域名判断）不会重复加",
				).Clicked() {
					// 一个都没加，说明内置的都已经在列表里了，也要说一声。
					n := a.store.RestoreBuiltins()
					if n > 0 {
						_ = a.store.Save()
						a.toast("已恢复 " + itoa(n) + " 个内置聊天")
					} else {
						a.toast("内置聊天都在列表里了")
					}
					a.win.Invalidate()
				}
				if ui.Button(c, "关闭").Clicked() {
					a.library = false
				}
			})
		})
	})
}

// libraryRow builds one chat of the library.
func (a *app) libraryRow(c *ui.Context, ch *Chat) {
	t := c.Theme()
	row := ui.Row(c).Key("lib:"+ch.ID).Padding(6, 8).Gap(10).Radius(8).AlignItems(ui.Center)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	if row.Clicked() {
		a.library = false
		a.activate(ch.ID)
	}
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item("打开").Chosen() {
			a.library = false
			a.activate(ch.ID)
		}
		if m.Item("编辑聊天…").Chosen() {
			a.openEditor(ch.ID, tabBasic)
		}
		pin := a.pinAction(ch)
		if m.Item(pin.label).Chosen() {
			pin.run()
			if !ch.Pinned {
				a.library = false
			}
		}
		m.Separator()
		if m.Item("从列表中删除").Chosen() {
			a.deleteChat(ch.ID)
		}
	})
	row.Children(func() {
		a.avatar(c, ch, 28)
		ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, ch.Name)
			ui.Text(c, hostOf(ch.URL)).FontSize(11).TextColor(t.TextMuted)
		})
		if a.store.IsOpen(ch.ID) {
			ui.Text(c, "已打开").FontSize(11).TextColor(t.TextMuted)
		}
		// 每行最后一个按钮：改名字、网址、图标、代理、CSS / JS。
		edit := ui.Row(c).Size(26, 26).Radius(6).AlignItems(ui.Center).Justify(ui.Center)
		edit.Label("编辑 " + ch.Name)
		edit.Tooltip("编辑 " + ch.Name)
		if edit.Hovered() {
			edit.Background(t.Surface)
		}
		if edit.Clicked() {
			a.library = false
			a.openEditor(ch.ID, tabBasic)
		}
		edit.Children(func() {
			ui.Icon(c, editIcon).Size(15, 15).TextColor(t.TextMuted)
		})
	})
}

// avatar builds a chat's icon, or the first letter of its name on a color
// the name picks while it has none.
func (a *app) avatar(c *ui.Context, ch *Chat, size float32) {
	if pic := a.icons.get(ch.Icon); pic != nil {
		ui.Image(c, pic).Size(size, size).Fit(ui.Contain).Radius(size / 2)
		return
	}
	ui.Row(c).Size(size, size).Radius(size / 2).
		Justify(ui.Center).AlignItems(ui.Center).
		Background(initialColor(ch.Name)).Children(func() {
		ui.Text(c, initialOf(ch.Name)).FontSize(size * 0.44).Bold().
			TextColor(ui.RGBA(255, 255, 255, 1))
	})
}

// editorView builds the editor of a chat: its name, URL and icon, and the
// CSS and JS the app injects into its pages.
func (a *app) editorView(c *ui.Context) {
	if !a.editor {
		return
	}
	t := c.Theme()
	d := &a.draft
	title := "新增聊天"
	if a.editingID != "" && a.store.Find(a.editingID) != nil {
		title = "编辑聊天"
	}
	ui.Modal(c, &a.editor, func() {
		ui.Column(c).Width(560).Gap(12).Children(func() {
			ui.Text(c, title).FontSize(18).Bold()
			ui.Tabs(c, &a.editorTab, "基本信息", "代理", "CSS 注入", "JS 注入")
			switch a.editorTab {
			case tabProxy:
				a.editorProxy(c)
			case tabCSS:
				a.editorCSS(c)
			case tabJS:
				a.editorJS(c)
			default:
				a.editorBasic(c, t)
			}
			switch {
			case d.Error != "":
				ui.Text(c, d.Error).FontSize(12).TextColor(t.Danger)
			case d.Message != "":
				ui.Text(c, d.Message).FontSize(12).TextColor(t.TextMuted)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					a.editor = false
				}
				if ui.PrimaryButton(c, "保存").Clicked() {
					a.saveDraft()
				}
			})
		})
	})
}

// editorBasic builds the editor's name, URL and icon.
func (a *app) editorBasic(c *ui.Context, t *ui.Theme) {
	d := &a.draft
	ui.Column(c).Gap(10).Children(func() {
		ui.TextInput(c, &d.Name).Label("名称").Placeholder("聊天名称").AutoFocus()
		ui.Field(c, "网址", func() {
			ui.Row(c).Gap(6).Children(func() {
				ui.TextInput(c, &d.URL).Placeholder("https://chat.example.com").Grow(1)
				label := "识别"
				if d.Detecting {
					label = "识别中…"
				}
				if ui.Button(c, label).Disabled(d.Detecting).Tooltip("按网址识别名称和图标").Clicked() {
					a.detectDraft()
				}
			})
		})
		ui.Field(c, "图标", func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				if d.IconPic != nil {
					ui.Image(c, d.IconPic).Size(44, 44).Fit(ui.Contain).Radius(12)
				} else {
					ui.Row(c).Size(44, 44).Radius(12).Border(1, t.Border).
						Justify(ui.Center).AlignItems(ui.Center).Children(func() {
						ui.Text(c, initialOf(d.Name)).FontSize(18).TextColor(t.TextMuted)
					})
				}
				ui.Column(c).Gap(6).Children(func() {
					ui.Row(c).Gap(6).Children(func() {
						if ui.Button(c, "上传图标").Clicked() {
							a.uploadIcon()
						}
						if ui.Button(c, "清除").Disabled(d.IconName == "").Clicked() {
							a.clearIcon()
						}
					})
					ui.Text(c, "可从网址识别，或上传 PNG、SVG、GIF、ICO").FontSize(11).TextColor(t.TextMuted)
				})
			})
		})
		ui.Collapsible(c, "高级选项", &d.Advanced, func() {
			ui.Column(c).Gap(10).Children(func() {
				ui.Field(c, "User-Agent", func() {
					ui.TextInput(c, &d.UserAgent).Placeholder("留空使用系统默认").Grow(1)
				})
				ui.Field(c, "缩放", func() {
					ui.Select(c, &d.Zoom, []string{"80%", "90%", "100%", "110%", "125%", "150%"}).Width(120)
				})
				ui.Checkbox(c, &d.Isolated, "独立会话：Cookie 与缓存和其他聊天分开")
				ui.Checkbox(c, &d.AllFrames, "CSS 与 JS 也注入页面里的 iframe")
			})
		})
	})
}

// editorProxy builds the editor's proxy: how the chat's pages reach the
// network, and the hosts that skip its proxy.
func (a *app) editorProxy(c *ui.Context) {
	d := &a.draft
	t := c.Theme()
	ui.Column(c).Gap(10).Children(func() {
		ui.Field(c, "代理", func() {
			ui.Select(c, &d.ProxyMode, proxyModes).Width(160)
		})
		if d.ProxyMode == proxyModes[2] {
			ui.Field(c, "代理地址", func() {
				ui.TextInput(c, &d.ProxyURL).
					Placeholder("http://127.0.0.1:7890 或 socks5://127.0.0.1:1080").Grow(1)
			})
			ui.Field(c, "不走代理", func() {
				ui.TextArea(c, &d.Bypass).Height(80).Lines(2, 6).
					Placeholder("每行一个域名，子域也算，例如\nlocalhost\n*.我司内网")
			})
		}
		ui.Text(c, "这个聊天页面里的所有请求都从这份代理出去，第三方域名（CDN、登录、"+
			"接口）也在内；别的聊天各按各的设置。需要 macOS 14 或更新。").
			FontSize(11).TextColor(t.TextMuted)
		if d.ProxyMode != proxyModes[0] {
			ui.Text(c, "走代理（或直连）的聊天会换成它自己的会话：Cookie 和登录状态与其它"+
				"聊天分开，原来共享会话里的登录看不到，需要在这个聊天里重新登录一次。").
				FontSize(11).TextColor(t.TextMuted)
		}
	})
}

// editorCSS builds the editor's style sheet.
func (a *app) editorCSS(c *ui.Context) {
	d := &a.draft
	t := c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, "加载页面时注入下面的 CSS，可用于隐藏元素、调整布局。").
			FontSize(12).TextColor(t.TextMuted)
		ui.TextArea(c, &d.CSS).Height(300).Font("monospace").
			Placeholder("/* 例如 */\nheader, nav { display: none !important; }")
	})
}

// editorJS builds the editor's script.
func (a *app) editorJS(c *ui.Context) {
	d := &a.draft
	t := c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, "页面就绪（DOMContentLoaded）后在页面中运行下面的 JS。").
			FontSize(12).TextColor(t.TextMuted)
		ui.TextArea(c, &d.JS).Height(300).Font("monospace").
			Placeholder("// 例如：自动展开输入框\nconst box = document.querySelector('textarea');\nbox && box.focus();")
	})
}

// initialOf returns the first letter of a name, upper-cased if it is a
// letter.
func initialOf(name string) string {
	for _, r := range strings.TrimSpace(name) {
		return string(unicode.ToUpper(r))
	}
	return "?"
}

// avatarColors color the icons of chats that have none, each name keeping
// its color.
var avatarColors = []ui.Color{
	ui.RGBA(0x5b, 0x8d, 0xef, 1),
	ui.RGBA(0x8a, 0x63, 0xe8, 1),
	ui.RGBA(0xd9, 0x6b, 0x9a, 1),
	ui.RGBA(0xe0, 0x8a, 0x3e, 1),
	ui.RGBA(0x2f, 0xa8, 0x7a, 1),
	ui.RGBA(0x35, 0x9c, 0xc4, 1),
	ui.RGBA(0x8d, 0x6e, 0x4f, 1),
	ui.RGBA(0x6b, 0x7a, 0x95, 1),
}

// initialColor picks the color of a chat's initial.
func initialColor(name string) ui.Color {
	var h uint32 = 2166136261
	for _, r := range name {
		h ^= uint32(r)
		h *= 16777619
	}
	return avatarColors[h%uint32(len(avatarColors))]
}

// itoa formats n for a message.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
