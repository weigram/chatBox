package main

// The app's development aid: any CHATBOX_DEBUG_* variable makes it open
// the first chat of the dock, wait for its page, report what the page says,
// and quit — with a screenshot, a dialog shown, a proxy of its own, and so
// on, as the variables ask:
//
//	CHATBOX_DEBUG_CAPTURE=shot.png   save a screenshot of the window's UI
//	CHATBOX_DEBUG_PAGE=page.png      save a screenshot of the page itself
//	CHATBOX_DEBUG_URL=https://…      the address the first chat opens
//	CHATBOX_DEBUG_DIALOG=library     show a dialog first: library, editor,
//	                                   css, js, proxy, settings
//	CHATBOX_DEBUG_INJECT=1           give the chat CSS and JS to inject
//	CHATBOX_DEBUG_PROXY=http://…     give the chat a proxy ("direct" too)
//	CHATBOX_DEBUG_SECOND=1           open a second chat, with no proxy
//	CHATBOX_DEBUG_TOGGLE=1           check the shortcut, the window and
//	                                   always-on-top, and a wrong accelerator
//	CHATBOX_DEBUG_MENU=1             log a page's context menu as it opens
//	CHATBOX_DEBUG_RECORD=Super+K     record that shortcut in the settings,
//	                                   which shows the question when taken
//	CHATBOX_DEBUG_LOG=path           write the log to this file, which a
//	                                   check can read while the app runs
//
//	go run .
//
// The app uses none of it without the variables.

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// report logs what a chat's page says: its title and URL, what it was
// given as a proxy, and what the app injected into it.
func report(a *app, label string, ch *Chat) {
	a.win.Update(func() {
		v := a.viewOf(ch)
		if v == nil {
			log.Printf("debug: %s %s has no web view", label, ch.Name)
			return
		}
		title, err := v.Page().Eval("document.title")
		href, _ := v.Page().Eval("location.href")
		log.Printf("debug: %s title=%v href=%v err=%v proxy=%q session=%v",
			label, title, href, err, ch.Proxy, a.sessions[ch.ID] != nil)
		style, _ := v.Page().Eval("(document.getElementById('mychatbox-style') || {}).textContent || ''")
		probe, _ := v.Page().Eval("window.__mychatboxProbe || ''")
		log.Printf("debug: %s css=%q js=%v", label, style, probe)
	})
}

// debugWanted reports whether a check asked for anything, which is when
// the app runs one.
func debugWanted() bool {
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "CHATBOX_DEBUG_") && name != "CHATBOX_DEBUG_LOG" {
			return true
		}
	}
	return false
}

// debugRun drives the app through a run, as a smoke test.
func (a *app) debugRun() {
	target := envVar("DEBUG_CAPTURE")
	dialog := envVar("DEBUG_DIALOG")
	url := envVar("DEBUG_URL")
	proxy := envVar("DEBUG_PROXY")
	inject := envVar("DEBUG_INJECT") != ""

	go func() {
		time.Sleep(2 * time.Second)

		chats := a.store.OpenChats()
		if len(chats) == 0 {
			log.Printf("debug: no chat in the dock")
			mygo.App.Quit()
			return
		}
		test := chats[0]
		if url != "" {
			test.URL = url
		}
		if inject {
			test.CSS = "body { outline: 6px solid rgb(255, 0, 0) !important; }"
			test.JS = "window.__mychatboxProbe = 'ok';"
		}
		if proxy != "" {
			test.Proxy = proxy
		}
		if url != "" || inject || proxy != "" {
			_ = a.store.Save()
		}
		log.Printf("debug: opening %s <%s>", test.Name, test.URL)
		a.activate(test.ID)

		// CHATBOX_DEBUG_CLOSE=1 closes the window on its own and reports
		// what became of it and of the app: a close puts the window away
		// rather than quitting, so that the global shortcut keeps working.
		if envVar("DEBUG_CLOSE") != "" {
			go func() {
				time.Sleep(2 * time.Second)
				a.win.Close()
				time.Sleep(1500 * time.Millisecond)
				log.Printf("debug: after close: window visible=%v destroyed=%v, shortcut=%q on=%v",
					a.win.IsVisible(), a.win.IsDestroyed(), a.store.Shortcut, a.store.ShortcutOn)
				mygo.App.Quit()
			}()
		}

		// CHATBOX_DEBUG_RECORD=Super+NoSuchKey opens the settings and
		// records that accelerator, as a key press would: with one the
		// system refuses, the conflict question shows.
		if rec := envVar("DEBUG_RECORD"); rec != "" {
			a.win.Update(func() {
				a.openSettings()
				a.recorded(rec)
			})
		}

		if dialog != "" {
			a.win.Update(func() {
				switch dialog {
				case "library":
					a.library = true
				case "editor":
					a.openEditor("", tabBasic)
				case "css":
					a.openEditor(a.store.Active, tabCSS)
				case "js":
					a.openEditor(a.store.Active, tabJS)
				case "settings":
					a.openSettings()
				case "proxy":
					a.openEditor(a.store.Active, tabProxy)
				}
			})
		}

		time.Sleep(10 * time.Second)
		report(a, "first", test)

		// CHATBOX_DEBUG_ACTION=reload reloads the page the way the menu
		// does, which is what a menu choice failing to act looked like.
		// CHATBOX_DEBUG_ACTION=devtools opens the inspector the way the
		// menu does, and says whether the page has it open.
		if envVar("DEBUG_ACTION") == "devtools" {
			a.devTools(test)
			time.Sleep(2 * time.Second)
			open := false
			if v := a.viewOf(test); v != nil {
				open = v.Page().IsDevToolsOpened()
			}
			log.Printf("debug: devtools open=%v", open)
		}

		if envVar("DEBUG_ACTION") == "reload" {
			log.Printf("debug: reloading %s", test.Name)
			a.reloadChat(test)
			time.Sleep(4 * time.Second)
			report(a, "after reload", test)
		}

		// A second chat, with no proxy of its own, says whether the proxy
		// of the first one reaches only it.
		if second := envVar("DEBUG_SECOND"); second != "" && len(chats) > 1 {
			other := chats[1]
			if url != "" {
				other.URL = url
			}
			_ = a.store.Save()
			log.Printf("debug: opening %s <%s>, no proxy", other.Name, other.URL)
			a.activate(other.ID)
			time.Sleep(10 * time.Second)
			report(a, "second", other)
		}

		if page := envVar("DEBUG_PAGE"); page != "" {
			if ch := a.store.Find(a.store.Active); ch != nil {
				if v := a.viewOf(ch); v != nil {
					if png, err := v.CapturePage(); err != nil {
						log.Printf("debug: page capture: %v", err)
					} else if err := os.WriteFile(page, png, 0o644); err != nil {
						log.Printf("debug: write %s: %v", page, err)
					} else {
						log.Printf("debug: wrote the page to %s, %d bytes", page, len(png))
					}
				}
			}
		}
		if envVar("DEBUG_TOGGLE") != "" {
			log.Printf("debug: shortcut=%q, stored=%q (on=%v)", a.shortcutDone, a.store.Shortcut, a.store.ShortcutOn)
			log.Printf("debug: visible=%v minimized=%v active=%v alwaysOnTop=%v (stored %v)",
				a.win.IsVisible(), a.win.IsMinimized(), mygo.App.IsActive(), a.win.IsAlwaysOnTop(), a.store.AlwaysOnTop)
			a.win.Hide()
			log.Printf("debug: after hide, visible=%v", a.win.IsVisible())
			a.toggleWindow()
			log.Printf("debug: after toggle, visible=%v active=%v", a.win.IsVisible(), mygo.App.IsActive())
			a.store.AlwaysOnTop = true
			a.applyAlwaysOnTop()
			log.Printf("debug: after set, alwaysOnTop=%v", a.win.IsAlwaysOnTop())
			// An accelerator the system does not take keeps the dialog
			// open with what went wrong.
			a.openSettings()
			a.setShortcut, a.setShortcutOn = "Nope+Key", true
			a.saveSettings()
			log.Printf("debug: bad shortcut -> message=%q error=%v dialogOpen=%v", a.settingsMsg, a.settingsErr, a.settings)
		}
		time.Sleep(1 * time.Second)
		if target != "" {
			png, err := a.win.CapturePage()
			if err != nil {
				log.Printf("debug: capture: %v", err)
			} else if err := os.WriteFile(target, png, 0o644); err != nil {
				log.Printf("debug: write %s: %v", target, err)
			} else {
				log.Printf("debug: wrote %s, %d bytes", target, len(png))
			}
		}
		mygo.App.Quit()
	}()
}
