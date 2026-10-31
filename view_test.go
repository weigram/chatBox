package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// testApp makes the app of a store with the builtin chats in a temporary
// data directory.
func testApp(t *testing.T) *app {
	t.Helper()
	dir := t.TempDir()
	store, err := loadStore(dir)
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	return &app{
		store:     store,
		icons:     newIconStore(filepath.Join(dir, "icons")),
		dataDir:   dir,
		views:     map[string]*mygo.WebView{},
		pageURL:   map[string]string{},
		sessions:  map[string]*mygo.Session{},
		iconTried: map[string]bool{},
	}
}

// TestFirstRunSeedsChats checks the builtin chats of a first run: they have
// IDs of their own, and the dock holds some.
func TestFirstRunSeedsChats(t *testing.T) {
	a := testApp(t)
	if len(a.store.All()) < 10 {
		t.Fatalf("got %d builtin chats", len(a.store.All()))
	}
	seen := map[string]bool{}
	for _, ch := range a.store.All() {
		if ch.ID == "" {
			t.Fatalf("%s has no ID", ch.Name)
		}
		if seen[ch.ID] {
			t.Fatalf("two chats share the ID %q", ch.ID)
		}
		seen[ch.ID] = true
		if !strings.HasPrefix(ch.URL, "https://") {
			t.Errorf("%s has the URL %q", ch.Name, ch.URL)
		}
	}
	if len(a.store.OpenChats()) == 0 {
		t.Error("the dock starts empty")
	}
}

// TestStoreRoundTrip checks that a store reads back what it saved.
func TestStoreRoundTrip(t *testing.T) {
	a := testApp(t)
	ch := &Chat{Name: "本地", URL: "https://example.com/chat", CSS: "body{}", JS: "1;"}
	a.store.Add(ch)
	a.store.SetActive(ch.ID)
	if err := a.store.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	again, err := loadStore(a.dataDir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got := again.Find(ch.ID)
	if got == nil {
		t.Fatal("the chat did not come back")
	}
	if got.Name != "本地" || got.URL != ch.URL || got.CSS != "body{}" || got.JS != "1;" {
		t.Errorf("got %+v", got)
	}
	if again.Active != ch.ID {
		t.Errorf("active %q, want %q", again.Active, ch.ID)
	}
}

// TestMoveLegacyDataDir checks the move the app makes when its name changed:
// the directory it kept under the old name becomes the one it uses now, so
// that a rename is not a fresh install.
func TestMoveLegacyDataDir(t *testing.T) {
	base := t.TempDir()
	legacy := filepath.Join(base, "MyChatBox")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(legacy, "chats.json")
	if err := os.WriteFile(marker, []byte(`{"seeded":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	fresh := filepath.Join(base, "chatBox")
	moveLegacyDataDir(fresh)
	if _, err := os.Stat(filepath.Join(fresh, "chats.json")); err != nil {
		t.Fatalf("the chats did not move: %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Error("the old directory is still there")
	}

	// 新目录已经在了就不动它（免得把新数据覆盖成老的）。
	other := filepath.Join(base, "second")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	moveLegacyDataDir(other)
	if _, err := os.Stat(other); err != nil {
		t.Errorf("the directory that exists was moved: %v", err)
	}

	// 目录名本来就是老名字时什么都不做。
	moveLegacyDataDir(legacy)
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Error("a legacy path was moved onto itself")
	}
}

// TestStoreMigrations checks a file of an older app: it gets the global
// shortcut once, and a shortcut the user clears stays cleared.
func TestStoreMigrations(t *testing.T) {
	dir := t.TempDir()
	old := `{"chats":[{"id":"a","name":"旧聊天","url":"https://example.com/"}],` +
		`"open":["a"],"active":"a","seeded":true}`
	if err := os.WriteFile(filepath.Join(dir, "chats.json"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := loadStore(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.Shortcut != defaultShortcut {
		t.Errorf("shortcut %q, want %q", s.Shortcut, defaultShortcut)
	}
	if s.ShortcutOn {
		t.Error("the shortcut came back on, and it is off by default")
	}
	if s.Schema != 2 {
		t.Errorf("schema %d, want 2", s.Schema)
	}
	if ch := s.Find("a"); ch == nil || ch.Name != "旧聊天" {
		t.Fatal("the old chats did not come back")
	}

	s.Shortcut, s.ShortcutOn = "", false
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	again, err := loadStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if again.Shortcut != "" || again.ShortcutOn {
		t.Errorf("the cleared shortcut came back as %q on=%v", again.Shortcut, again.ShortcutOn)
	}
}

// stubProbe replaces the system check of an accelerator for a test, and
// puts the real one back when the test ends.
func stubProbe(t *testing.T, fn func(string) error) {
	t.Helper()
	old := probe
	probe = fn
	t.Cleanup(func() { probe = old })
}

// TestAcceleratorOf checks what a key press becomes: the accelerator's
// canonical text, and nothing for a key with no modifier.
func TestAcceleratorOf(t *testing.T) {
	cases := []struct {
		ev   ui.InputEvent
		want string
	}{
		{ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyM, Mods: ui.Super | ui.Shift}, "Super+Shift+M"},
		{ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeySpace, Mods: ui.Alt}, "Alt+Space"},
		{ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyF5, Mods: ui.Ctrl | ui.Alt}, "Ctrl+Alt+F5"},
		{ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeySlash, Mods: ui.Super}, "Super+/"},
		// No modifier: a global shortcut would take the key from every app.
		{ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyM}, ""},
		// A key up, or a key held down, is none either.
		{ui.InputEvent{Kind: ui.InputKeyUp, Key: ui.KeyM, Mods: ui.Super}, ""},
		{ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyM, Mods: ui.Super, Repeat: true}, ""},
	}
	for _, c := range cases {
		got, ok := acceleratorOf(c.ev)
		if c.want == "" {
			if ok {
				t.Errorf("%+v became %q, want nothing", c.ev, got)
			}
			continue
		}
		if !ok || got != c.want {
			t.Errorf("%+v became %q (%v), want %q", c.ev, got, ok, c.want)
		}
	}
}

// TestDisplayAccel checks the accelerator as the settings show it.
func TestDisplayAccel(t *testing.T) {
	// CmdOrCtrl is the ⌘ of macOS and the ⌃ of the others, as it is in mygo.
	cmd := "⌘"
	if runtime.GOOS != "darwin" {
		cmd = "⌃"
	}
	for spec, want := range map[string]string{
		"":                  "未设置",
		"Super+Shift+M":     "⌘⇧M",
		"Ctrl+Alt+Space":    "⌃⌥空格",
		"Super+Shift+Slash": "⌘⇧Slash",
		"CmdOrCtrl+Shift+M": cmd + "⇧M",
	} {
		if got := displayAccel(spec); got != want {
			t.Errorf("%q shows as %q, want %q", spec, got, want)
		}
	}
}

// TestRecordedTakesOrAsks checks what recording a shortcut does: one the
// system takes is kept, and one it refuses asks first.
func TestRecordedTakesOrAsks(t *testing.T) {
	a := testApp(t)
	// Another application holds the accelerator: the system refuses it.
	stubProbe(t, func(string) error { return errors.New("already registered") })

	a.recording = true
	a.recorded("Super+Shift+K")
	if !a.confirmShortcut {
		t.Fatalf("a refused accelerator asked nothing")
	}
	if a.setShortcut == "Super+Shift+K" {
		t.Error("the refused accelerator was kept before the answer")
	}
	if a.recError == "" {
		t.Error("nothing says why it was refused")
	}

	// Saying yes keeps the key, and leaves the shortcut off: turning it on
	// asks about the conflict again.
	a.keepRecorded()
	if a.setShortcut != "Super+Shift+K" {
		t.Errorf("saying yes gave %q", a.setShortcut)
	}
	if a.setShortcutOn {
		t.Error("saying yes turned the shortcut on")
	}
	if a.recording || a.confirmShortcut {
		t.Error("the recorder or the question stayed open")
	}
}

// TestRecordKey checks the keys the recorder handles on its own.
func TestRecordKey(t *testing.T) {
	a := testApp(t)
	a.recording = true
	a.setShortcut, a.setShortcutOn = defaultShortcut, true

	// ⌫ clears the shortcut, and stops the recording.
	if !a.recordKey(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyBackspace}) {
		t.Fatal("the recorder let ⌫ through")
	}
	if a.setShortcut != "" || a.setShortcutOn || a.recording {
		t.Errorf("⌫ gave %q on=%v recording=%v", a.setShortcut, a.setShortcutOn, a.recording)
	}

	// Esc puts the recording away and keeps what there was.
	a.recording, a.setShortcut = true, defaultShortcut
	if !a.recordKey(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEscape}) {
		t.Fatal("the recorder let Esc through")
	}
	if a.recording || a.setShortcut != defaultShortcut {
		t.Errorf("Esc gave recording=%v, shortcut %q", a.recording, a.setShortcut)
	}

	// A key with no modifier is refused, with a word about it.
	a.recording = true
	if !a.recordKey(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyM}) {
		t.Fatal("the recorder let a bare key through")
	}
	if a.recError == "" || a.setShortcut != defaultShortcut {
		t.Errorf("a bare key gave %q, error %q", a.setShortcut, a.recError)
	}
}

// TestRecorderTakesKeys checks the recorder through the view: a click on
// the box listens, and the keys pressed after it become the shortcut.
func TestRecorderTakesKeys(t *testing.T) {
	a := testApp(t)
	stubProbe(t, func(string) error { return nil })
	a.openSettings()
	tt := ui.NewTester(a.view, 1200, 800)

	if err := tt.Click("快捷键"); err != nil {
		t.Fatalf("click the recorder: %v", err)
	}
	tt.Frame()
	if !a.recording {
		t.Fatal("the box did not start recording")
	}
	tt.Key(ui.Super|ui.Shift, ui.KeyK)
	tt.Frame()
	if a.recording {
		t.Error("the recorder kept listening after a key")
	}
	if a.setShortcut != "Super+Shift+K" {
		t.Errorf("the recording gave %q", a.setShortcut)
	}
	if a.setShortcutOn {
		t.Error("a new shortcut came on by itself")
	}
}

// TestRecordingTurnsTheShortcutOff checks that a new key is not in use
// until it is turned on, and that recording the same key changes nothing.
func TestRecordingTurnsTheShortcutOff(t *testing.T) {
	a := testApp(t)
	stubProbe(t, func(string) error { return nil })
	a.setShortcut, a.setShortcutOn = "Super+Shift+M", true

	// The same key again: it stays on.
	a.recorded("Super+Shift+M")
	if !a.setShortcutOn {
		t.Error("recording the key in use turned the shortcut off")
	}

	// A different key: off, until the switch says otherwise.
	a.recorded("Super+Shift+K")
	if a.setShortcut != "Super+Shift+K" || a.setShortcutOn {
		t.Errorf("a new key gave %q on=%v", a.setShortcut, a.setShortcutOn)
	}
}

// TestEnablingChecksTheKey checks the switch: it turns the shortcut on only
// when the system takes the key, and asks when it does not.
func TestEnablingChecksTheKey(t *testing.T) {
	a := testApp(t)
	a.setShortcut, a.setShortcutOn = "", false

	// No key to turn on.
	a.tryEnable()
	if a.setShortcutOn || a.recError == "" {
		t.Errorf("no key gave on=%v, error %q", a.setShortcutOn, a.recError)
	}

	// A key the system takes.
	stubProbe(t, func(string) error { return nil })
	a.setShortcut = "Super+Shift+K"
	a.tryEnable()
	if !a.setShortcutOn {
		t.Error("a free key did not turn the shortcut on")
	}

	// A key another application holds: asked about, and off until answered.
	stubProbe(t, func(string) error { return errors.New("already registered") })
	a.setShortcutOn = false
	a.tryEnable()
	if a.setShortcutOn {
		t.Error("a held key turned the shortcut on without asking")
	}
	if !a.confirmShortcut || !a.confirmForEnable {
		t.Errorf("nothing asked: confirm=%v forEnable=%v", a.confirmShortcut, a.confirmForEnable)
	}
	a.enableRecorded()
	if !a.setShortcutOn || a.confirmShortcut {
		t.Errorf("saying yes gave on=%v, question %v", a.setShortcutOn, a.confirmShortcut)
	}

	// Saying no keeps it off.
	a.setShortcutOn = false
	a.tryEnable()
	a.confirmShortcut, a.confirmForEnable = false, false
	if a.setShortcutOn {
		t.Error("saying no turned the shortcut on")
	}
}

// TestSwitchAsksAboutTheKey checks the switch in the view: turning it on
// with a key the system refuses asks before anything happens.
func TestSwitchAsksAboutTheKey(t *testing.T) {
	a := testApp(t)
	a.openSettings()
	a.setShortcut, a.setShortcutOn = "Super+Shift+K", false
	stubProbe(t, func(string) error { return errors.New("already registered") })

	tt := ui.NewTester(a.view, 1200, 800)
	if err := tt.Click("启用快捷键"); err != nil {
		t.Fatalf("click the switch: %v", err)
	}
	tt.Frame()
	if a.setShortcutOn {
		t.Error("the switch turned a held key on")
	}
	if !a.confirmShortcut || !a.confirmForEnable {
		t.Errorf("the switch asked nothing: %v %v", a.confirmShortcut, a.confirmForEnable)
	}
	if !tt.HasText("这个快捷键用不了") {
		t.Errorf("the question does not say what happened: %q", tt.Texts())
	}
}

// TestPinToDock checks the menu item that keeps a chat in the dock: it
// pins, opens, and comes back as the one that lets it go.
func TestPinToDock(t *testing.T) {
	a := testApp(t)
	ch := a.store.All()[0]

	act := a.pinAction(ch)
	if act.label != "固定到 Dock 栏" {
		t.Fatalf("a loose chat offers %q", act.label)
	}
	act.run()
	if !ch.Pinned || !a.store.IsOpen(ch.ID) {
		t.Errorf("pinning gave pinned=%v open=%v", ch.Pinned, a.store.IsOpen(ch.ID))
	}
	if again, err := loadStore(a.dataDir); err != nil || !again.Find(ch.ID).Pinned {
		t.Errorf("the pin did not reach the file: %v", err)
	}

	// 固定之后，菜单里那一项变成“取消固定”。
	act = a.pinAction(ch)
	if act.label != "取消固定" {
		t.Fatalf("a pinned chat offers %q", act.label)
	}
	act.run()
	if ch.Pinned {
		t.Error("unpinning left it pinned")
	}
}

// TestDockGroupsPinnedFirst checks the order the dock shows: the pinned
// chats, then the open ones, the two groups apart.
func TestDockGroupsPinnedFirst(t *testing.T) {
	a := testApp(t)
	all := a.store.All()
	// 固定的聊天排在列表的后面，好看出分组不是照列表顺序来的。
	pinned := all[len(all)-1]
	a.store.Pin(pinned.ID)

	got, rest := a.store.DockChats()
	if len(got) != 1 || got[0].ID != pinned.ID {
		t.Fatalf("the pinned group is %v", got)
	}
	for _, ch := range rest {
		if ch.ID == pinned.ID {
			t.Error("the pinned chat is in the open group too")
		}
	}

	// 关掉标签页，固定的那个还留着。
	a.store.Close(pinned.ID)
	got, _ = a.store.DockChats()
	if len(got) != 1 {
		t.Errorf("closing the tab took the pinned chat away: %v", got)
	}
}

// TestMovePinned checks the order the pinned chats keep: dragging one up
// or down changes it, and it stays within the group.
func TestMovePinned(t *testing.T) {
	a := testApp(t)
	all := a.store.All()
	first, second, third := all[0], all[1], all[2]
	for _, ch := range []*Chat{first, second, third} {
		a.store.Pin(ch.ID)
	}
	order := func() string {
		var ids []string
		for _, ch := range a.store.PinnedChats() {
			ids = append(ids, ch.Name)
		}
		return strings.Join(ids, ">")
	}
	if got, want := order(), first.Name+">"+second.Name+">"+third.Name; got != want {
		t.Fatalf("pinning gave %s, want %s", got, want)
	}

	if !a.store.MovePinned(third.ID, -1) {
		t.Fatal("a move of one place did nothing")
	}
	if got, want := order(), first.Name+">"+third.Name+">"+second.Name; got != want {
		t.Errorf("up one gave %s, want %s", got, want)
	}

	// 拖过头就停在两端。
	if !a.store.MovePinned(third.ID, -9) {
		t.Error("a move past the top did nothing")
	}
	if got := order(); !strings.HasPrefix(got, third.Name) {
		t.Errorf("moving past the top gave %s", got)
	}
	a.store.MovePinned(third.ID, 9)
	if got := order(); !strings.HasSuffix(got, third.Name) {
		t.Errorf("moving past the bottom gave %s", got)
	}

	// 没挪动就说没挪动。
	if a.store.MovePinned(third.ID, 0) {
		t.Error("a move of no places said it moved")
	}

	// 顺序写进文件。
	a.saveStore()
	if again, err := loadStore(a.dataDir); err != nil {
		t.Fatal(err)
	} else if got := len(again.PinOrder); got != 3 {
		t.Errorf("the file holds %d pinned ids, want 3", got)
	}
}

// TestDockDragReorders checks the drag itself, through the view: dragging a
// pinned icon up moves it up the dock.
func TestDockDragReorders(t *testing.T) {
	a := testApp(t)
	open := a.store.OpenChats()
	if len(open) < 3 {
		t.Skip("this store opens too few chats")
	}
	// 固定前三个，第四个拖到最上面。
	for _, ch := range open[:3] {
		a.store.Pin(ch.ID)
	}
	top := open[0]
	last := open[2]
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()

	box, ok := tt.Find("打开 " + last.Name)
	if !ok {
		t.Fatalf("no dock icon for %s: %q", last.Name, tt.Texts())
	}
	x, y := box.X+box.W/2, box.Y+box.H/2
	tt.Press(x, y)
	for i := 1; i <= 6; i++ {
		tt.Move(x, y-float32(i)*15)
	}
	tt.Release(x, y-90)
	tt.Frame()

	if got := a.store.PinnedChats()[0].ID; got != last.ID {
		t.Errorf("the drag left %v on top, want %v", got, last.ID)
	}
	if got := a.store.PinnedChats()[1].ID; got != top.ID {
		t.Errorf("the next one is %v, want %v", got, top.ID)
	}
}

// TestMoveOpenTo checks moving an open chat along the list, which is what
// dragging a tab does.
func TestMoveOpenTo(t *testing.T) {
	a := testApp(t)
	open := a.store.OpenChats()
	if len(open) < 3 {
		t.Skip("this store opens too few chats")
	}
	first, third := open[0], open[2]
	if !a.store.MoveOpenTo(third.ID, 0) {
		t.Fatal("a move to the front did nothing")
	}
	got := a.store.OpenChats()
	if got[0].ID != third.ID || got[1].ID != first.ID {
		t.Errorf("moving to the front gave %v %v", got[0].Name, got[1].Name)
	}
	// 位置越界、原地不动都不算移动。
	if a.store.MoveOpenTo(third.ID, len(open)) {
		t.Error("a move past the end said it moved")
	}
	if a.store.MoveOpenTo(third.ID, 0) {
		t.Error("a move to where it is said it moved")
	}

	a.saveStore()
	if again, err := loadStore(a.dataDir); err != nil {
		t.Fatal(err)
	} else if again.Open[0] != third.ID {
		t.Errorf("the file keeps %q first, want %q", again.Open[0], third.ID)
	}
}

// TestTabDragReorders checks the drag itself: dragging a tab over another
// one puts it there.
func TestTabDragReorders(t *testing.T) {
	a := testApp(t)
	open := a.store.OpenChats()
	if len(open) < 3 {
		t.Skip("this store opens too few chats")
	}
	first := open[0]
	third := open[2]
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()

	from, ok := tt.Find(first.Name)
	if !ok {
		t.Fatalf("no tab named %s: %q", first.Name, tt.Texts())
	}
	to, ok := tt.Find(third.Name)
	if !ok {
		t.Fatalf("no tab named %s", third.Name)
	}
	x0, y0 := from.X+from.W/2, from.Y+from.H/2
	x1 := to.X + to.W/2
	// 拖的过程中不要再手动 Frame：Tester 的每次事件本就会推动帧，
	// 多插一帧会把这一帧的位移吃掉。
	tt.Press(x0, y0)
	for i := 1; i <= 6; i++ {
		tt.Move(x0+(x1-x0)*float32(i)/6, y0)
	}
	tt.Release(x1, y0)
	tt.Frame()

	// 第一个标签落到第三个位置上：它现在排第三，原来的第二个顶上来。
	if got := a.store.OpenChats()[2].ID; got != first.ID {
		t.Errorf("the third tab is %q, want %q", got, first.ID)
	}
	if got := a.store.OpenChats()[0].ID; got == first.ID {
		t.Error("the dragged tab stayed first")
	}
}

// TestPinLeadsWithTheWindowButtons checks which side the pin takes: the one
// the window's own buttons leave free. macOS keeps the traffic lights on the
// left, Windows the minimise, maximise and close on the right.
func TestPinLeadsWithTheWindowButtons(t *testing.T) {
	for _, c := range []struct {
		name string
		bar  ui.TitleBar
		want bool
	}{
		{"macOS", ui.TitleBar{Height: 28, Left: 61}, false},
		{"Windows", ui.TitleBar{Height: 32, Right: 138}, true},
		{"no buttons", ui.TitleBar{}, false},
	} {
		if got := pinLeads(c.bar); got != c.want {
			t.Errorf("%s: pinLeads=%v, want %v", c.name, got, c.want)
		}
	}
}

// TestBuiltinConflict checks the shortcuts of the window the app warns
// about before the system does.
func TestBuiltinConflict(t *testing.T) {
	if got := builtinConflict("Super+N"); got == "" {
		t.Error("⌘N is no conflict")
	}
	if got := builtinConflict("Super+Shift+M"); got != "" {
		t.Errorf("%q is a conflict: %s", "Super+Shift+M", got)
	}
}

// TestSettingsKeepsTheShortcut checks the settings dialog: it shows what
// the app has, and what it saves is what the app keeps.
func TestSettingsKeepsTheShortcut(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1200, 800)
	if a.store.ShortcutOn || a.store.Shortcut != defaultShortcut {
		t.Fatalf("the first run has %q on=%v, want the key offered, off",
			a.store.Shortcut, a.store.ShortcutOn)
	}
	a.openSettings()
	tt.Frame()
	// 设置里只有这几项；窗口置顶只在标题栏那个图钉上。
	for _, want := range []string{"设置", "全局快捷键", "显示地址栏", "搜索用网址", "数据目录"} {
		if !tt.HasText(want) {
			t.Errorf("the settings dialog does not show %q: %q", want, tt.Texts())
		}
	}

	// 录制/改成新键位后，启用是关着的；打开它（系统接受）才会生效。
	stubProbe(t, func(string) error { return nil })
	a.setShortcut = "Alt+Shift+C"
	a.tryEnable()
	if !a.setShortcutOn {
		t.Fatal("enabling a free shortcut left it off")
	}
	tt.Click("保存")
	tt.Frame()
	if a.settings {
		t.Error("the settings dialog stayed open")
	}
	if a.store.Shortcut != "Alt+Shift+C" || !a.store.ShortcutOn {
		t.Errorf("saved %q on=%v", a.store.Shortcut, a.store.ShortcutOn)
	}
	if again, err := loadStore(a.dataDir); err != nil || again.Shortcut != "Alt+Shift+C" {
		t.Errorf("the shortcut did not reach the file: %q %v", again.Shortcut, err)
	}

	// Turning the switch off keeps the keys, and stops using them.
	a.openSettings()
	a.setShortcutOn = false
	tt.Frame()
	tt.Click("保存")
	tt.Frame()
	if a.store.ShortcutOn {
		t.Error("the shortcut is still on")
	}
	if a.store.Shortcut != "Alt+Shift+C" {
		t.Errorf("the keys were lost: %q", a.store.Shortcut)
	}
}

// TestAlwaysOnTopChecksTheStore checks the toggle the toolbar and the
// settings share.
func TestAlwaysOnTopChecksTheStore(t *testing.T) {
	a := testApp(t)
	a.store.AlwaysOnTop = true
	if err := a.store.Save(); err != nil {
		t.Fatal(err)
	}
	if again, err := loadStore(a.dataDir); err != nil || !again.AlwaysOnTop {
		t.Errorf("always on top did not reach the file: %v %v", again.AlwaysOnTop, err)
	}
}

// TestDockSwitchesChats checks the dock: a click on an icon shows that
// chat, and the toolbar names it.
func TestDockSwitchesChats(t *testing.T) {
	a := testApp(t)
	open := a.store.OpenChats()
	a.store.SetActive(open[0].ID)

	tt := ui.NewTester(a.view, 1200, 800)
	if !tt.HasText(open[0].Name) {
		t.Fatalf("the toolbar does not name %s: %q", open[0].Name, tt.Texts())
	}
	// The dock's bottom holds the settings button alone: a chat is added
	// from the tab strip's +, or ⌘N. Both are icon buttons, which the
	// tester finds by their label, as the icons have no text.
	if box, ok := tt.Find("设置：全局快捷键、地址栏与数据目录"); !ok {
		t.Errorf("no settings button in the dock: %q", tt.Texts())
	} else if box.X > dockWidth {
		t.Errorf("the settings button is at x=%v, outside the dock", box.X)
	}
	if box, ok := tt.Find("新增聊天"); !ok {
		t.Errorf("no add button in the tab strip: %q", tt.Texts())
	} else if box.X < dockWidth {
		t.Errorf("the add button is at x=%v, in the dock", box.X)
	}
	// The chats' names are on their tabs, in the window's title bar.
	if box, ok := tt.Find(open[0].Name); !ok {
		t.Error("no tab names the chat")
	} else if box.Y > 44 {
		t.Errorf("the tab is at y=%v, below the title bar", box.Y)
	}
	if err := tt.Click("打开 " + open[0].Name); err != nil {
		t.Fatalf("the dock's icon of %s: %v", open[0].Name, err)
	}
	if err := tt.Click("打开 " + open[1].Name); err != nil {
		t.Fatalf("the dock's icon of %s: %v", open[1].Name, err)
	}
	if a.store.Active != open[1].ID {
		t.Errorf("the dock did not switch chats, showing %q", a.store.Active)
	}
	tt.Frame()
	if !tt.HasText(open[1].Name) {
		t.Errorf("no tab names %s: %q", open[1].Name, tt.Texts())
	}
}

// TestDockSizesFit checks that the biggest circle of the dock fits inside
// it: a ring wider than the column leaves would be cut at its edges, which
// is what happened when the dock was 58 wide with 6 of padding and a 48
// ring.
func TestDockSizesFit(t *testing.T) {
	const padding = float32(4)
	if got, want := dockRing+2*padding, dockWidth; got > want {
		t.Errorf("the circle, %.0f with the padding, is wider than the dock's %.0f", got, want)
	}
	if dockIcon >= dockRing {
		t.Errorf("the icon (%.0f) leaves no ring in the circle (%.0f)", dockIcon, dockRing)
	}
}

// TestChatMenuItems checks the one list of a chat's menu: its view-side
// menu and the native one the toolbar's ⋯ pops up hold the same items.
func TestChatMenuItems(t *testing.T) {
	a := testApp(t)
	ch := a.store.All()[0]

	native := a.chatMenuNative(ch)
	got := labelsList(native)
	want := []string{
		"编辑聊天…", "固定到 Dock 栏", "---",
		"重新载入", "开发者工具", "在浏览器中打开", "清空缓存…", "---",
		"关闭标签页",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("the menu holds\n  %v\nwant\n  %v", got, want)
	}

	// The same menu on a tab, through the view: right-clicking it lists
	// the same labels.
	tt := ui.NewTester(a.view, 1200, 800)
	if err := tt.RightClick(ch.Name); err != nil {
		t.Fatalf("right-click the chat's tab: %v", err)
	}
	shown := tt.Menu()
	if len(shown) != len(want) {
		t.Fatalf("the tab's menu holds %v, want %v", shown, want)
	}
	for i, label := range want {
		if label != "---" && shown[i] != label {
			t.Errorf("item %d is %q, want %q", i, shown[i], label)
		}
	}
}

// TestTabsSwitchAndClose checks the tab strip: a click on a tab shows that
// chat, and a click on the close button beside it takes it out of the dock.
func TestTabsSwitchAndClose(t *testing.T) {
	a := testApp(t)
	open := a.store.OpenChats()
	a.store.SetActive(open[0].ID)
	tt := ui.NewTester(a.view, 1200, 800)

	for _, ch := range open[:3] {
		if !tt.HasText(ch.Name) {
			t.Fatalf("no tab named %s: %q", ch.Name, tt.Texts())
		}
	}
	if box, ok := tt.Find(open[1].Name); !ok {
		t.Fatal("no box for the second tab")
	} else if box.Y > 38 {
		t.Errorf("the tab is at y=%v, not in the strip", box.Y)
	}

	if err := tt.Click(open[1].Name); err != nil {
		t.Fatalf("click the tab: %v", err)
	}
	if a.store.Active != open[1].ID {
		t.Errorf("the tab did not switch chats, showing %q", a.store.Active)
	}

	if err := tt.Click("关闭 " + open[1].Name); err != nil {
		t.Fatalf("click the close button: %v", err)
	}
	if a.store.IsOpen(open[1].ID) {
		t.Error("the close button did not close the tab")
	}
	if a.store.Active != open[0].ID {
		t.Errorf("after closing, the app shows %q", a.store.Active)
	}
}

// TestAddressBarShowsTheAddress checks the address bar: the toolbar is off
// by default, and once on it shows the page's own address, ready to edit.
func TestAddressBarShowsTheAddress(t *testing.T) {
	a := testApp(t)
	ch := a.store.All()[0]
	a.store.SetActive(ch.ID)
	a.pageURL[ch.ID] = "https://chat.example.com/c/42"

	tt := ui.NewTester(a.view, 1200, 800)
	// 后退按钮是那一行独有的，用它判断整行在不在。
	if tt.HasText("后退") {
		t.Error("the toolbar shows while it is off")
	}

	a.store.ShowToolbar = true
	tt.Frame()
	if !tt.HasText("后退") {
		t.Fatalf("the toolbar is not drawn: %q", tt.Texts())
	}
	if a.addrText != "https://chat.example.com/c/42" {
		t.Errorf("the address bar reads %q", a.addrText)
	}
}

// TestAddressBarEdits checks that the address bar opens for typing when
// clicked, and that what is typed stays there instead of being overwritten
// by the page's own address.
func TestAddressBarEdits(t *testing.T) {
	a := testApp(t)
	a.store.ShowToolbar = true
	ch := a.store.All()[0]
	a.store.SetActive(ch.ID)
	a.pageURL[ch.ID] = "https://chat.example.com/"
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()

	if err := tt.Click("地址"); err != nil {
		t.Fatalf("click the address bar: %v", err)
	}
	tt.Frame()
	if !a.addrFocused {
		t.Fatal("clicking the address bar did not open it for typing")
	}
	tt.Type("/new")
	tt.Frame()
	if !strings.HasSuffix(a.addrText, "/new") {
		t.Errorf("what was typed reads %q", a.addrText)
	}
	// 页面自己的地址不会再盖掉正在输入的内容。
	a.pageURL[ch.ID] = "https://changed.example/"
	tt.Frame()
	if !strings.HasSuffix(a.addrText, "/new") {
		t.Errorf("the page's address overwrote the typing: %q", a.addrText)
	}
}

// TestAddressTarget checks what the address bar does with what was typed.
func TestAddressTarget(t *testing.T) {
	a := testApp(t)
	for typed, want := range map[string]string{
		"example.com":         "https://example.com",
		"https://a.example/x": "https://a.example/x",
		"http://b.example/":   "http://b.example/",
	} {
		if got := a.addressTarget(typed); got != want {
			t.Errorf("%q became %q, want %q", typed, got, want)
		}
	}
	if got := a.addressTarget("今天的天气"); !strings.HasPrefix(got, "https://") {
		t.Errorf("words became %q, want a search", got)
	}
}

// TestLibraryEditButton checks the pencil at the end of a chat's row: it
// opens that chat's editor, and puts the list away.
func TestLibraryEditButton(t *testing.T) {
	a := testApp(t)
	ch := a.store.All()[0]
	a.library = true
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()

	if err := tt.Click("编辑 " + ch.Name); err != nil {
		t.Fatalf("click the edit button: %v", err)
	}
	tt.Frame()
	if a.library {
		t.Error("the list stayed open over the editor")
	}
	if !a.editor {
		t.Fatal("the edit button opened no editor")
	}
	if a.draft.ID != ch.ID {
		t.Errorf("the editor edits %q, want %q", a.draft.ID, ch.ID)
	}
}

// TestToolbarSetting checks the switch that shows the row of page controls:
// off at first, kept in the file, and drawn or not as it says.
func TestToolbarSetting(t *testing.T) {
	a := testApp(t)
	if a.store.ShowToolbar {
		t.Error("the toolbar is on at first")
	}
	a.openSettings()
	tt := ui.NewTester(a.view, 1200, 800)
	// 先出一帧：没画过就没有可点的位置。
	tt.Frame()
	if err := tt.Click("显示地址栏"); err != nil {
		t.Fatalf("click the switch: %v", err)
	}
	tt.Frame()
	if !a.setToolbar {
		t.Fatal("the switch did not turn the toolbar on")
	}
	tt.Click("保存")
	tt.Frame()
	if !a.store.ShowToolbar {
		t.Error("saving kept the toolbar off")
	}
	if again, err := loadStore(a.dataDir); err != nil || !again.ShowToolbar {
		t.Errorf("the setting did not reach the file: %v %v", again.ShowToolbar, err)
	}
	// 地址栏画出来了：那一行跟着 store 走。
	if !tt.HasText("地址") {
		t.Errorf("the toolbar is not drawn: %q", tt.Texts())
	}
}

// TestEditorSavesAChat checks the editor: a URL without a scheme gets one,
// a chat without a name takes its host's, and the CSS is kept.
func TestEditorSavesAChat(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1200, 800)

	a.openEditor("", 0)
	tt.Frame()
	if !tt.HasText("新增聊天") {
		t.Fatalf("the editor did not open: %q", tt.Texts())
	}

	// Without a URL the editor keeps a message, not a chat.
	tt.Click("保存")
	tt.Frame()
	if got, want := len(a.store.All()), len(builtinChats()); got != want {
		t.Fatalf("the editor saved a chat without a URL: %d chats", got)
	}
	if a.draft.Error == "" {
		t.Error("the editor shows no message for a missing URL")
	}

	// The fields change the draft, as typing does.
	a.draft.Name = "我的聊天"
	a.draft.URL = "example.com/chat"
	a.draft.CSS = "body { color: red; }"
	a.draft.JS = "window.x = 1;"
	a.draft.Zoom = "125%"
	tt.Frame()
	tt.Click("保存")
	tt.Frame()

	ch := a.store.Find(a.store.Active)
	if ch == nil {
		t.Fatal("no chat was saved")
	}
	if ch.Name != "我的聊天" {
		t.Errorf("name %q", ch.Name)
	}
	if ch.URL != "https://example.com/chat" {
		t.Errorf("URL %q", ch.URL)
	}
	if !strings.Contains(ch.CSS, "color: red") || ch.JS != "window.x = 1;" {
		t.Errorf("CSS %q, JS %q", ch.CSS, ch.JS)
	}
	if ch.Zoom != 1.25 {
		t.Errorf("zoom %v", ch.Zoom)
	}
	if !a.store.IsOpen(ch.ID) {
		t.Error("the saved chat is not in the dock")
	}

	// A chat without a name takes the host of its URL.
	a.openEditor("", 0)
	a.draft.URL = "https://chat.example.org/"
	a.saveDraft()
	named := a.store.Find(a.store.Active)
	if named == nil || named.Name != "chat.example.org" {
		t.Errorf("name %q, want the host", named.Name)
	}
}

// TestClosingTheChatOpenShowsAnother checks that closing the chat the app
// shows picks another one of the dock, and that the last one leaves none.
func TestClosingTheChatOpenShowsAnother(t *testing.T) {
	a := testApp(t)
	open := a.store.OpenChats()
	first, second := open[0], open[1]
	a.store.SetActive(first.ID)

	a.closeChat(first.ID)
	if a.store.Active != second.ID {
		t.Fatalf("after closing %s the app shows %q, want %s", first.Name, a.store.Active, second.Name)
	}
	if a.store.IsOpen(first.ID) {
		t.Error("the closed chat is still in the dock")
	}

	for _, id := range append([]string{}, a.store.Open...) {
		a.closeChat(id)
	}
	if a.store.Active != "" {
		t.Errorf("the app still shows %q with an empty dock", a.store.Active)
	}
	if len(a.store.OpenChats()) != 0 {
		t.Error("the dock is not empty")
	}
	if len(a.store.All()) != len(builtinChats()) {
		t.Error("closing a chat removed it from the library")
	}
}

// TestValidateProxy checks what the editor takes as a proxy: the three
// modes, an HTTP or SOCKS5 proxy with its port, and nothing else.
func TestValidateProxy(t *testing.T) {
	for _, spec := range []string{
		"", "direct",
		"http://127.0.0.1:7890",
		"https://proxy.lan:8443",
		"http://proxy.lan",
		"socks5://127.0.0.1:1080",
		"socks5h://user:secret@127.0.0.1:1080",
	} {
		if err := validateProxy(spec); err != nil {
			t.Errorf("%q was refused: %v", spec, err)
		}
	}
	for _, spec := range []string{
		"socks5://127.0.0.1",     // no port
		"ftp://127.0.0.1:21",     // not a proxy Go or the system speaks
		"http://",                // no host
		"127.0.0.1:7890",         // no scheme
		"http://proxy.lan:99999", // no such port
		"http://proxy.lan:abc",   // not a port
	} {
		if err := validateProxy(spec); err == nil {
			t.Errorf("%q was taken for a proxy", spec)
		}
	}
}

// TestProxyModes checks what the editor's modes and its text mean.
func TestProxyModes(t *testing.T) {
	if got := proxySpecOf(proxyModes[0], "http://x:1"); got != "" {
		t.Errorf("跟随系统 saved %q", got)
	}
	if got := proxySpecOf(proxyModes[1], "http://x:1"); got != proxyDirect {
		t.Errorf("直连 saved %q", got)
	}
	if got := proxySpecOf(proxyModes[2], " http://x:1 "); got != "http://x:1" {
		t.Errorf("自定义 saved %q", got)
	}
	for spec, want := range map[string]string{
		"":           proxyModes[0],
		proxyDirect:  proxyModes[1],
		"http://x:1": proxyModes[2],
	} {
		if got := proxyModeOf(spec); got != want {
			t.Errorf("%q read as %q, want %q", spec, got, want)
		}
	}
	got := parseBypass("localhost\n*.lan, example.com;foo.com\n\n bar.com")
	want := []string{"localhost", "*.lan", "example.com", "foo.com", "bar.com"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("bypass %q, want %q", got, want)
	}
	if got := describeProxyFully("http://x:1"); !strings.Contains(got, "http://x:1") {
		t.Errorf("a proxy reads as %q", got)
	}
	if got := describeProxyFully(""); !strings.Contains(got, "系统") {
		t.Errorf("no proxy reads as %q", got)
	}
}

// TestEditorSavesTheProxy checks the editor's proxy tab: what it saves,
// and that a proxy the system cannot parse keeps the dialog open on it.
func TestEditorSavesTheProxy(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1200, 800)

	a.openEditor("", tabProxy)
	tt.Frame()
	if !tt.HasText("代理") || !tt.HasText("跟随系统") {
		t.Fatalf("the proxy tab does not show: %q", tt.Texts())
	}
	a.draft.Name = "走代理的聊天"
	a.draft.URL = "https://chat.example.com/"
	a.draft.ProxyMode = proxyModes[2]
	a.draft.ProxyURL = "http://127.0.0.1:7890"
	a.draft.Bypass = "localhost\n*.lan"
	tt.Frame()
	tt.Click("保存")
	tt.Frame()

	ch := a.store.Find(a.store.Active)
	if ch == nil {
		t.Fatal("no chat was saved")
	}
	if ch.Proxy != "http://127.0.0.1:7890" {
		t.Errorf("proxy %q", ch.Proxy)
	}
	if strings.Join(ch.ProxyBypass, " ") != "localhost *.lan" {
		t.Errorf("bypass %q", ch.ProxyBypass)
	}

	// A proxy the system cannot parse keeps the dialog open, on its tab.
	a.openEditor(ch.ID, tabBasic)
	a.draft.ProxyMode = proxyModes[2]
	a.draft.ProxyURL = "socks5://127.0.0.1"
	tt.Frame()
	tt.Click("保存")
	tt.Frame()
	if !a.editor {
		t.Error("the editor closed on a proxy that cannot work")
	}
	if a.editorTab != tabProxy {
		t.Errorf("the editor stayed on tab %d, want the proxy one", a.editorTab)
	}
	if a.draft.Error == "" {
		t.Error("the editor shows no message for a bad proxy")
	}
	if saved := a.store.Find(ch.ID); saved.Proxy != "http://127.0.0.1:7890" {
		t.Errorf("the bad proxy reached the chat: %q", saved.Proxy)
	}
}

// TestProxyOfAChatIsKept checks that a chat's proxy and its bypass list
// reach the file, and come back.
func TestProxyOfAChatIsKept(t *testing.T) {
	a := testApp(t)
	ch := &Chat{
		Name: "代理聊天", URL: "https://example.com/",
		Proxy:       "socks5://user:pw@127.0.0.1:1080",
		ProxyBypass: []string{"localhost", "*.lan"},
	}
	a.store.Add(ch)
	a.store.SetActive(ch.ID)
	if err := a.store.Save(); err != nil {
		t.Fatal(err)
	}
	again, err := loadStore(a.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	got := again.Find(ch.ID)
	if got == nil || got.Proxy != ch.Proxy {
		t.Fatalf("proxy came back as %+v", got)
	}
	if strings.Join(got.ProxyBypass, " ") != "localhost *.lan" {
		t.Errorf("bypass %q", got.ProxyBypass)
	}
}

// TestTextFragment checks the link the menu copies for a selection: the
// page's URL with a text fragment, which browsers highlight.
func TestTextFragment(t *testing.T) {
	if got := textFragment("https://example.com/doc#part2", "月度均价 测算"); got !=
		"https://example.com/doc#:~:text=%E6%9C%88%E5%BA%A6%E5%9D%87%E4%BB%B7%20%E6%B5%8B%E7%AE%97" {
		t.Errorf("fragment %q", got)
	}
	if got := textFragment("", "x"); got != "" {
		t.Errorf("no page reads as %q", got)
	}
	if got := textFragment("https://example.com/", "  "); got != "https://example.com/" {
		t.Errorf("no text reads as %q", got)
	}
	long := strings.Repeat("字", 200)
	got := textFragment("https://example.com/", long)
	if !strings.HasPrefix(got, "https://example.com/#:~:text=") {
		t.Fatalf("long fragment %q", got)
	}
	encoded := strings.TrimPrefix(got, "https://example.com/#:~:text=")
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		t.Fatalf("the fragment does not decode: %v", err)
	}
	if runes := len([]rune(decoded)); runes != 120 {
		t.Errorf("the fragment holds %d runes of the text, want 120", runes)
	}
}

// TestSearchURL checks the URL the menu searches with, from the settings'
// template.
func TestSearchURL(t *testing.T) {
	a := testApp(t)
	if got := a.searchURL("月度 均价"); got != "https://cn.bing.com/search?q=%E6%9C%88%E5%BA%A6+%E5%9D%87%E4%BB%B7" {
		t.Errorf("default search %q", got)
	}
	a.store.SearchURL = "https://www.baidu.com/s?wd=%s&ie=utf-8"
	if got := a.searchURL("abc"); got != "https://www.baidu.com/s?wd=abc&ie=utf-8" {
		t.Errorf("own search %q", got)
	}
	// A template with nowhere to put the words falls back to the app's.
	a.store.SearchURL = "https://example.com/none"
	if got := a.searchURL("abc"); !strings.HasPrefix(got, "https://cn.bing.com/search?q=") {
		t.Errorf("a template without %s gave %q", "%s", got)
	}
}

// TestPageMenu checks the menu a page shows on a right-click: the web
// view's items stay, and a browser's are added, without doubling the ones
// the web view already has.
func TestPageMenu(t *testing.T) {
	a := testApp(t)
	ch := a.store.All()[0]
	a.pageURL[ch.ID] = "https://chat.example.com/c/1"

	// What WebKit gives for a selection: copying, looking up.
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{ID: mygo.ContextMenuCopy, Label: "拷贝"},
		{ID: mygo.ContextMenuLookUp, Label: "查询"},
	})
	e := &mygo.ContextMenuEvent{
		SelectionText: "支持用户选择不同套餐测算月度均价",
		LinkURL:       "https://chat.example.com/plan",
		Menu:          menu,
	}
	a.pageMenuFor(e, ch, nil)

	labels := map[string]bool{}
	for _, it := range menu.Items() {
		if it.Label != "" {
			labels[it.Label] = true
		}
	}
	for _, want := range []string{
		"重新载入", "复制所选中内容的链接", "在浏览器中搜索“支持用户选择不同套餐测算月度均价”",
		"复制链接地址", "用默认浏览器打开链接", "复制页面地址", "用默认浏览器打开当前页", "打印…",
	} {
		if !labels[want] {
			t.Errorf("the menu has no %q: %v", want, labelsList(menu))
		}
	}
	if labels["复制选中文字"] {
		t.Error("the menu copies the selection twice: WebKit's item was there")
	}
	if menu.ItemByID(mygo.ContextMenuCopy) == nil {
		t.Error("WebKit's own copy item is gone")
	}
	// The web view gave no navigation of its own, so the app's go first.
	if first := menu.Items()[0]; first.ID != "mychatbox.back" {
		t.Errorf("the menu begins with %q, want the navigation items", first.Label)
	}

	// A menu of the web view's own that already has them: the app adds no
	// second copy of any of it.
	full := mygo.NewMenu([]*mygo.MenuItem{
		{ID: mygo.ContextMenuBack, Label: "返回"},
		{ID: mygo.ContextMenuForward, Label: "前进"},
		{ID: mygo.ContextMenuReload, Label: "重新载入"},
		{ID: mygo.ContextMenuCopy, Label: "拷贝"},
		{ID: mygo.ContextMenuCopyLink, Label: "拷贝链接"},
		{ID: mygo.ContextMenuPrint, Label: "打印"},
		{ID: mygo.ContextMenuInspect, Label: "检查元素"},
	})
	e2 := &mygo.ContextMenuEvent{SelectionText: "x", Menu: full}
	a.pageMenuFor(e2, ch, nil)
	count := 0
	for _, it := range full.Items() {
		if it.ID == "mychatbox.copyLink" || it.ID == "mychatbox.print" || it.ID == "mychatbox.inspect" {
			count++
		}
	}
	if count != 0 {
		t.Errorf("the app added %d items the web view already had: %v", count, labelsList(full))
	}

	// No selection, no link: the page's own address is still offered.
	plain := mygo.NewMenu(nil)
	a.pageMenuFor(&mygo.ContextMenuEvent{Menu: plain}, ch, nil)
	if plain.ItemByID("mychatbox.copyPage") == nil {
		t.Errorf("no copy of the page address: %v", labelsList(plain))
	}
	if plain.ItemByID("mychatbox.copySelectionLink") != nil {
		t.Error("the menu offers a selection link with nothing selected")
	}
}

// TestLibraryListsChats checks the library dialog lists every chat.
func TestLibraryListsChats(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1200, 800)
	a.library = true
	tt.Frame()
	if !tt.HasText("聊天列表") {
		t.Fatalf("the library did not open: %q", tt.Texts())
	}
	if !tt.HasText("恢复内置聊天") {
		t.Error("no way to restore the builtin chats")
	}
	tt.Click("关闭")
	tt.Frame()
	if a.library {
		t.Error("the library did not close")
	}
}

// TestPreloadInjects checks the script a chat's pages get: its CSS and JS
// as JavaScript literals, and the hook the app changes them through.
func TestPreloadInjects(t *testing.T) {
	ch := &Chat{ID: "abc", CSS: "body { content: \"a\" }\n", JS: `console.log("</script>")`}
	script := preloadScript(ch)
	if !strings.Contains(script, jsLiteral(ch.CSS)) {
		t.Error("the CSS is not in the script")
	}
	if !strings.Contains(script, "__mychatboxApply") {
		t.Error("the script has no hook to change the CSS")
	}
	apply := applyScript(ch, true)
	if !strings.HasSuffix(apply, "true)") {
		t.Errorf("apply does not run the JS: %s", apply)
	}
	if !strings.Contains(apply, `\"a\"`) {
		t.Errorf("the CSS is not encoded for JavaScript: %s", apply)
	}
}

// TestDetectMetaReadsATitleAndIcon checks the name and icon a page gives,
// with the site served by the test.
func TestDetectMetaReadsATitleAndIcon(t *testing.T) {
	icon := []byte("\x89PNG\r\n\x1a\nnot really a png")
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!doctype html><html><head>
			<title> 火苗 Chat | 对话 </title>
			<link rel="icon" type="image/png" sizes="64x64" href="/static/icon.png">
		</head><body>hi</body></html>`))
	})
	mux.HandleFunc("/static/icon.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(icon)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	meta, err := detectMeta(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if meta.Title != "火苗 Chat | 对话" {
		t.Errorf("title %q", meta.Title)
	}
	if meta.IconURL != srv.URL+"/static/icon.png" {
		t.Errorf("icon URL %q", meta.IconURL)
	}
	if len(meta.IconData) != len(icon) {
		t.Errorf("icon data %d bytes", len(meta.IconData))
	}
	if meta.IconExt != "png" {
		t.Errorf("icon extension %q", meta.IconExt)
	}
}

// TestChooseIconPrefersPNG checks the icon the app picks from a page's
// links: a bitmap over an .ico, and the favicon when the page links none.
func TestChooseIconPrefersPNG(t *testing.T) {
	base, err := parseURL("https://chat.example.com/app")
	if err != nil {
		t.Fatal(err)
	}
	got := chooseIcon([]iconLink{
		{href: "/favicon.ico", rel: "icon"},
		{href: "/icon.png", rel: "icon", typ: "image/png", sizes: "32x32"},
		{href: "/big.png", rel: "apple-touch-icon", sizes: "180x180"},
	}, base)
	if want := "https://chat.example.com/big.png"; got != want {
		t.Errorf("icon %q, want %q", got, want)
	}
	if got := chooseIcon(nil, base); got != "https://chat.example.com/favicon.ico" {
		t.Errorf("fallback icon %q", got)
	}
}

// TestParseURL checks the URL a chat is given.
func TestParseURL(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"chat.deepseek.com", "https://chat.deepseek.com"},
		{" https://example.com/a?b=c ", "https://example.com/a?b=c"},
		{"http://localhost:8080/x", "http://localhost:8080/x"},
	} {
		u, err := parseURL(c.in)
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if u.String() != c.want {
			t.Errorf("%q: got %q, want %q", c.in, u.String(), c.want)
		}
	}
	for _, in := range []string{"", "   ", "file:///etc/passwd", "javascript:alert(1)", "https://"} {
		if _, err := parseURL(in); err == nil {
			t.Errorf("%q was accepted", in)
		}
	}
}

// TestZoomLabels checks the zoom the editor shows and reads.
func TestZoomLabels(t *testing.T) {
	for _, zoom := range []float64{0.8, 1, 1.25} {
		label := zoomLabel(zoom)
		if got := zoomValue(label); got != zoom {
			t.Errorf("%v reads as %v through %q", zoom, got, label)
		}
	}
	if got := zoomValue("bad"); got != 1 {
		t.Errorf("bad label reads as %v", got)
	}
}
