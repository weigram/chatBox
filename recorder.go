package main

import (
	"log"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// 快捷键：录制、显示与冲突检查。
//
// 存下来的是 mygo 认的加速键字符串（"Super+Shift+M"），显示时换成
// macOS 的符号（"⌘⇧M"）。

// keyAccelNames names the keys an accelerator can end with.
var keyAccelNames = []struct {
	key  ui.Key
	name string
}{
	{ui.KeyA, "A"}, {ui.KeyB, "B"}, {ui.KeyC, "C"}, {ui.KeyD, "D"},
	{ui.KeyE, "E"}, {ui.KeyF, "F"}, {ui.KeyG, "G"}, {ui.KeyH, "H"},
	{ui.KeyI, "I"}, {ui.KeyJ, "J"}, {ui.KeyK, "K"}, {ui.KeyL, "L"},
	{ui.KeyM, "M"}, {ui.KeyN, "N"}, {ui.KeyO, "O"}, {ui.KeyP, "P"},
	{ui.KeyQ, "Q"}, {ui.KeyR, "R"}, {ui.KeyS, "S"}, {ui.KeyT, "T"},
	{ui.KeyU, "U"}, {ui.KeyV, "V"}, {ui.KeyW, "W"}, {ui.KeyX, "X"},
	{ui.KeyY, "Y"}, {ui.KeyZ, "Z"},
	{ui.Key0, "0"}, {ui.Key1, "1"}, {ui.Key2, "2"}, {ui.Key3, "3"},
	{ui.Key4, "4"}, {ui.Key5, "5"}, {ui.Key6, "6"}, {ui.Key7, "7"},
	{ui.Key8, "8"}, {ui.Key9, "9"},
	{ui.KeyF1, "F1"}, {ui.KeyF2, "F2"}, {ui.KeyF3, "F3"}, {ui.KeyF4, "F4"},
	{ui.KeyF5, "F5"}, {ui.KeyF6, "F6"}, {ui.KeyF7, "F7"}, {ui.KeyF8, "F8"},
	{ui.KeyF9, "F9"}, {ui.KeyF10, "F10"}, {ui.KeyF11, "F11"}, {ui.KeyF12, "F12"},
	{ui.KeySpace, "Space"}, {ui.KeyEnter, "Enter"}, {ui.KeyTab, "Tab"},
	{ui.KeyEscape, "Escape"}, {ui.KeyBackspace, "Backspace"}, {ui.KeyDelete, "Delete"},
	{ui.KeyUp, "Up"}, {ui.KeyDown, "Down"}, {ui.KeyLeft, "Left"}, {ui.KeyRight, "Right"},
	{ui.KeyHome, "Home"}, {ui.KeyEnd, "End"},
	{ui.KeyPageUp, "PageUp"}, {ui.KeyPageDown, "PageDown"},
	{ui.KeyMinus, "-"}, {ui.KeyEqual, "="}, {ui.KeyComma, ","}, {ui.KeyPeriod, "."},
	{ui.KeySlash, "/"}, {ui.KeySemicolon, ";"}, {ui.KeyQuote, "'"},
	{ui.KeyBracketLeft, "["}, {ui.KeyBracketRight, "]"}, {ui.KeyBackslash, `\`},
}

// keyAccelName returns the accelerator's name for a key, "" for one an
// accelerator cannot hold.
func keyAccelName(k ui.Key) string {
	for _, kv := range keyAccelNames {
		if kv.key == k {
			return kv.name
		}
	}
	return ""
}

// acceleratorOf makes the accelerator of the key just pressed: its
// modifiers and the key, in the canonical order mygo parses. A key with no
// modifier is none, since a global shortcut without one would take the key
// from every application.
func acceleratorOf(ev ui.InputEvent) (string, bool) {
	if ev.Kind != ui.InputKeyDown || ev.Repeat {
		return "", false
	}
	key := keyAccelName(ev.Key)
	if key == "" {
		key = keyAccelName(ev.PhysicalKey)
	}
	if key == "" {
		return "", false
	}
	// A modifier on its own is not a shortcut: pressing Shift reports the
	// key Shift with no modifiers, which names no accelerator key.
	if ev.Mods == 0 {
		return "", false
	}
	var parts []string
	if ev.Mods&ui.Super != 0 {
		parts = append(parts, "Super")
	}
	if ev.Mods&ui.Ctrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if ev.Mods&ui.Alt != 0 {
		parts = append(parts, "Alt")
	}
	if ev.Mods&ui.Shift != 0 {
		parts = append(parts, "Shift")
	}
	return strings.Join(append(parts, key), "+"), true
}

// displayAccel writes an accelerator as macOS shows it: ⌘⇧M. On other
// systems it keeps the names, since their keys have no such glyphs.
func displayAccel(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "未设置"
	}
	parts := strings.Split(spec, "+")
	var out strings.Builder
	key := parts[len(parts)-1]
	for _, part := range parts[:len(parts)-1] {
		switch part {
		case "Super":
			out.WriteString("⌘")
		case "Cmd":
			out.WriteString("⌘")
		case "CmdOrCtrl":
			// What mygo reads as Command on macOS and Control elsewhere.
			if runtime.GOOS == "darwin" {
				out.WriteString("⌘")
			} else {
				out.WriteString("⌃")
			}
		case "Ctrl":
			out.WriteString("⌃")
		case "Alt":
			out.WriteString("⌥")
		case "Shift":
			out.WriteString("⇧")
		default:
			out.WriteString(part + "+")
		}
	}
	if key == "Space" {
		key = "空格"
	}
	out.WriteString(key)
	return out.String()
}

// probe is probeShortcut, which tests replace: registering with the system
// needs the app's loop, and a view test runs without one.
var probe = probeShortcut

// probeShortcut checks an accelerator before the app commits to it: that
// mygo can parse it, and that the system takes it. It lets go of it right
// away, so the app registers it for real only once the user agrees.
func probeShortcut(spec string) error {
	probe := func() {}
	if err := mygo.GlobalShortcut.Register(spec, probe); err != nil {
		return err
	}
	mygo.GlobalShortcut.Unregister(spec)
	return nil
}

// builtinConflict names the shortcut of the window an accelerator would
// take, "" when it takes none of them.
func builtinConflict(spec string) string {
	switch spec {
	case "Super+N", "Ctrl+N":
		return "新建聊天（⌘N）"
	case "Super+L", "Ctrl+L":
		return "打开聊天列表（⌘L）"
	}
	return ""
}

// recordKey takes the keys pressed while the shortcut recorder waits. It
// reports whether the key was taken.
func (a *app) recordKey(ev ui.InputEvent) bool {
	if !a.recording || ev.Kind != ui.InputKeyDown {
		return false
	}
	switch {
	case ev.Key == ui.KeyEscape && ev.Mods == 0:
		a.recording, a.recError = false, ""
		return true
	case (ev.Key == ui.KeyBackspace || ev.Key == ui.KeyDelete) && ev.Mods == 0:
		// Delete clears the shortcut.
		a.setShortcut, a.setShortcutOn = "", false
		a.recording, a.recError = false, ""
		return true
	}
	spec, ok := acceleratorOf(ev)
	if !ok {
		if keyAccelName(ev.Key) == "" {
			return true // nothing an accelerator can hold
		}
		a.recError = "要带一个修饰键：⌘、⌥、⌃ 或 ⇧"
		return true
	}
	a.recorded(spec)
	return true
}

// recorded takes the accelerator just recorded: it is kept at once when
// the system takes it, and asked about when another application holds it.
// A new key comes with the shortcut off: turning it on is a step of its
// own, which checks the system took the key.
func (a *app) recorded(spec string) {
	a.recError = ""
	changed := spec != a.setShortcut
	// The accelerator the app has is the app's own: trying to register it
	// again would fail, and it is no conflict.
	var err error
	if spec != a.shortcutDone {
		err = probe(spec)
	}
	if err == nil {
		a.setShortcut = spec
		if changed {
			a.setShortcutOn = false
		}
		a.recording = false
		return
	}
	log.Printf("myChatBox: 快捷键 %s 注册不了：%v", spec, err)
	a.recPending = spec
	a.recError = err.Error()
	a.confirmShortcut = true
	if changed {
		a.setShortcutOn = false
	}
}

// keepRecorded keeps the accelerator the system refused anyway, once the
// user said so: it may be free later, when the other application quits.
// The shortcut stays off until it is turned on, which asks again.
func (a *app) keepRecorded() {
	a.setShortcut, a.setShortcutOn = a.recPending, false
	a.recording, a.recError = false, ""
	a.confirmShortcut, a.confirmForEnable = false, false
	a.saveSettings()
}

// tryEnable turns the shortcut on, once the system takes its key: one the
// system refuses is asked about first, since it would not work and another
// application most likely holds it.
func (a *app) tryEnable() {
	if a.setShortcut == "" {
		a.setShortcutOn = false
		a.recError = "先录一个快捷键，再打开启用"
		return
	}
	a.recError = ""
	var err error
	if a.setShortcut != a.shortcutDone {
		err = probe(a.setShortcut)
	}
	if err == nil {
		a.setShortcutOn = true
		return
	}
	log.Printf("myChatBox: 快捷键 %s 启用不了：%v", a.setShortcut, err)
	a.recPending = a.setShortcut
	a.recError = err.Error()
	a.setShortcutOn = false
	a.confirmForEnable = true
	a.confirmShortcut = true
}

// enableRecorded turns the shortcut on after the user said so, conflict
// and all: it takes effect when the other application lets the key go.
func (a *app) enableRecorded() {
	a.setShortcutOn = true
	a.recError = ""
	a.confirmShortcut, a.confirmForEnable = false, false
}

// conflictReason says why an accelerator was refused, in words for the
// dialog: another application holds it, most of the time.
func conflictReason(err string) string {
	switch {
	case strings.Contains(err, "already registered"):
		return "这个组合已经被 chatBox 自己注册了"
	case strings.Contains(err, "unknown modifier"), strings.Contains(err, "invalid"):
		return "这个组合 mygo 不认（" + err + "）"
	default:
		return "系统拒绝注册这个组合，通常是别的应用已经占用了它"
	}
}
