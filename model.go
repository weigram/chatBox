package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Chat is one web chat the app shows in its dock: a page the system's web
// view loads, the name and icon the dock shows, and what the app injects
// into every page of it.
type Chat struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
	// Icon is the file name of the chat's icon in the data directory's
	// icons, empty for none (the dock shows the name's initial).
	Icon string `json:"icon,omitempty"`
	// CSS is a style sheet injected into every page of the chat.
	CSS string `json:"css,omitempty"`
	// JS is a script run in every page of the chat, once it is ready.
	JS string `json:"js,omitempty"`
	// Zoom scales the chat's pages, 0 for 1.
	Zoom float64 `json:"zoom,omitempty"`
	// UserAgent overrides the browser's, empty for the system's.
	UserAgent string `json:"userAgent,omitempty"`
	// Proxy is how the chat's pages reach the network: empty for the
	// system's proxy, "direct" to connect directly whatever the system
	// says, or a proxy of its own as "http://127.0.0.1:7890" or
	// "socks5://127.0.0.1:1080". A chat with one uses a session of its
	// own, since the system takes proxies per store (macOS 14 and later).
	Proxy string `json:"proxy,omitempty"`
	// ProxyBypass are hosts, and their subdomains, the chat reaches
	// directly, whatever its proxy says.
	ProxyBypass []string `json:"proxyBypass,omitempty"`
	// Isolated gives the chat website data of its own: its cookies and
	// storage apart from the other chats'.
	Isolated bool `json:"isolated,omitempty"`
	// AllFrames injects CSS and JS into the page's iframes too.
	AllFrames bool `json:"allFrames,omitempty"`
	// Pinned keeps the chat in the dock: pinned chats come first, and stay
	// there even when their tab is closed.
	Pinned bool `json:"pinned,omitempty"`

	// Builtin marks the chats the app starts with.
	Builtin bool `json:"builtin,omitempty"`
}

// Store is the app's chats and the session of its dock, kept in chats.json.
type Store struct {
	dir   string
	Chats []*Chat `json:"chats"`
	// Open is the dock's chats, in the order it shows them.
	Open []string `json:"open"`
	// Active is the chat the app shows, empty for none.
	Active string `json:"active"`
	// Seeded tells whether the app made the builtin chats, which it does
	// once, on the first run.
	Seeded bool `json:"seeded"`
	// Shortcut shows and hides the window from anywhere, as an
	// accelerator such as "CmdOrCtrl+Shift+M", empty for none.
	Shortcut string `json:"shortcut,omitempty"`
	// ShowToolbar shows the row of page controls and the address above the
	// page. It is off by default: the window is for the page, which brings
	// its own toolbar; 设置 turns it on for the ones that do not.
	ShowToolbar bool `json:"showToolbar,omitempty"`

	// PinOrder is the order the pinned chats show in, which dragging them
	// up and down the dock changes.
	PinOrder []string `json:"pinOrder,omitempty"`

	// ShortcutOn tells whether the app registers Shortcut.
	ShortcutOn bool `json:"shortcutOn,omitempty"`
	// AlwaysOnTop keeps the window above the other applications'.
	AlwaysOnTop bool `json:"alwaysOnTop,omitempty"`
	// SearchURL searches the web for what a page's context menu asks,
	// with "%s" where the words go; empty for the app's own.
	SearchURL string `json:"searchUrl,omitempty"`
	// Schema is the version of the file, which migrations of the app that
	// read it from an older one look at.
	Schema int `json:"schema,omitempty"`
}

// defaultShortcut shows and hides the window from anywhere until the user
// changes it. It is a chord few applications claim.
const defaultShortcut = "CmdOrCtrl+Shift+M"

// newID returns a short random identifier.
func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("chat-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// loadStore reads the chats of dir, seeding the builtin ones on the first
// run.
func loadStore(dir string) (*Store, error) {
	s := &Store{dir: dir}
	data, err := os.ReadFile(filepath.Join(dir, "chats.json"))
	switch {
	case err == nil:
		if err := json.Unmarshal(data, s); err != nil {
			return nil, err
		}
		if s.Chats == nil {
			s.Chats = []*Chat{}
		}
		// A file of an app from before the global shortcut existed gets
		// it once; clearing it later keeps it cleared.
		if s.Schema < 1 {
			s.Schema = 1
			if s.Shortcut == "" && !s.ShortcutOn {
				s.Shortcut = defaultShortcut
			}
			if err := s.Save(); err != nil {
				return nil, err
			}
		}
		// The shortcut off by default: registering a key with the system
		// is something to ask for, not something to impose.
		if s.Schema < 2 {
			s.Schema = 2
			s.ShortcutOn = false
			if err := s.Save(); err != nil {
				return nil, err
			}
		}
	case errors.Is(err, os.ErrNotExist):
		s.Chats = builtinChats()
		for _, ch := range s.Chats {
			ch.ID = newID()
		}
		s.Seeded = true
		s.Schema = 2
		// The default key is offered, not registered: 设置 里的开关要人自己打开。
		s.Shortcut, s.ShortcutOn = defaultShortcut, false
		// The dock starts with the chats most people want, without
		// loading any page until one is chosen.
		for _, ch := range s.Chats[:min(4, len(s.Chats))] {
			s.Open = append(s.Open, ch.ID)
		}
		if err := s.Save(); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}
	return s, nil
}

// Save writes the store to chats.json, replacing the file in one step. It
// is safe from any goroutine.
func (s *Store) Save() error {
	if s.dir == "" {
		return nil
	}
	saveMu.Lock()
	defer saveMu.Unlock()
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(struct {
		Chats       []*Chat  `json:"chats"`
		Open        []string `json:"open"`
		Active      string   `json:"active"`
		Seeded      bool     `json:"seeded"`
		ShowToolbar bool     `json:"showToolbar,omitempty"`
		PinOrder    []string `json:"pinOrder,omitempty"`
		Shortcut    string   `json:"shortcut,omitempty"`
		ShortcutOn  bool     `json:"shortcutOn,omitempty"`
		AlwaysOnTop bool     `json:"alwaysOnTop,omitempty"`
		SearchURL   string   `json:"searchUrl,omitempty"`
		Schema      int      `json:"schema,omitempty"`
	}{s.Chats, s.Open, s.Active, s.Seeded, s.ShowToolbar, s.PinOrder, s.Shortcut,
		s.ShortcutOn, s.AlwaysOnTop, s.SearchURL, s.Schema}, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, "chats.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// saveMu serializes the writes of chats.json.
var saveMu sync.Mutex

// All returns the chats the library lists.
func (s *Store) All() []*Chat { return s.Chats }

// Find returns the chat of id, nil for none.
func (s *Store) Find(id string) *Chat {
	for _, ch := range s.Chats {
		if ch.ID == id {
			return ch
		}
	}
	return nil
}

// Add appends a chat to the library.
func (s *Store) Add(ch *Chat) {
	if ch.ID == "" {
		ch.ID = newID()
	}
	s.Chats = append(s.Chats, ch)
}

// Remove deletes a chat from the library and the dock.
func (s *Store) Remove(id string) {
	chats := s.Chats[:0]
	for _, ch := range s.Chats {
		if ch.ID != id {
			chats = append(chats, ch)
		}
	}
	s.Chats = chats
	s.Close(id)
	if s.Active == id {
		s.Active = ""
	}
}

// PinnedChats returns the chats kept in the dock, in the order the user
// dragged them into.
func (s *Store) PinnedChats() []*Chat {
	var out []*Chat
	for _, id := range s.pinnedOrder() {
		if ch := s.Find(id); ch != nil && ch.Pinned {
			out = append(out, ch)
		}
	}
	return out
}

// pinnedOrder is the pinned chats' ids in order: what the user dragged,
// with the pinned chats of an older file, or of a pin made without one,
// after them in the list's order.
func (s *Store) pinnedOrder() []string {
	out := make([]string, 0, len(s.PinOrder))
	seen := map[string]bool{}
	for _, id := range s.PinOrder {
		ch := s.Find(id)
		if ch == nil || !ch.Pinned || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	for _, ch := range s.Chats {
		if ch.Pinned && !seen[ch.ID] {
			out = append(out, ch.ID)
		}
	}
	return out
}

// MovePinned moves a pinned chat steps places up (negative) or down in the
// dock, and tells whether it went anywhere.
func (s *Store) MovePinned(id string, steps int) bool {
	order := s.pinnedOrder()
	from := -1
	for i, o := range order {
		if o == id {
			from = i
		}
	}
	if from < 0 {
		return false
	}
	to := from + steps
	if to < 0 {
		to = 0
	}
	if to > len(order)-1 {
		to = len(order) - 1
	}
	if to == from {
		return false
	}
	moved := order[from]
	order = append(order[:from], order[from+1:]...)
	order = append(order[:to], append([]string{moved}, order[to:]...)...)
	s.PinOrder = order
	return true
}

// DockChats returns what the dock shows: the pinned chats first, then the
// ones open without being pinned.
func (s *Store) DockChats() (pinned, rest []*Chat) {
	pinned = s.PinnedChats()
	for _, ch := range s.OpenChats() {
		if !ch.Pinned {
			rest = append(rest, ch)
		}
	}
	return pinned, rest
}

// Pin keeps a chat in the dock, and opens it.
func (s *Store) Pin(id string) {
	ch := s.Find(id)
	if ch == nil {
		return
	}
	ch.Pinned = true
	s.Dock(id)
	for _, o := range s.PinOrder {
		if o == id {
			return
		}
	}
	s.PinOrder = append(s.PinOrder, id)
}

// Unpin lets a chat leave the dock when its tab closes; it stays open.
func (s *Store) Unpin(id string) {
	ch := s.Find(id)
	if ch == nil {
		return
	}
	ch.Pinned = false
	out := s.PinOrder[:0]
	for _, o := range s.PinOrder {
		if o != id {
			out = append(out, o)
		}
	}
	s.PinOrder = out
}

// OpenChats returns the chats of the dock, in its order, skipping the ones
// the library no longer has.
func (s *Store) OpenChats() []*Chat {
	out := make([]*Chat, 0, len(s.Open))
	for _, id := range s.Open {
		if ch := s.Find(id); ch != nil {
			out = append(out, ch)
		}
	}
	return out
}

// IsOpen reports whether the dock holds the chat.
func (s *Store) IsOpen(id string) bool {
	for _, o := range s.Open {
		if o == id {
			return true
		}
	}
	return false
}

// Dock adds a chat to the dock, at the end.
func (s *Store) Dock(id string) {
	if !s.IsOpen(id) {
		s.Open = append(s.Open, id)
	}
}

// MoveOpenTo moves an open chat to a place in the dock and the tab strip,
// counting from the start; a place outside the list is no move.
func (s *Store) MoveOpenTo(id string, to int) bool {
	from := -1
	for i, o := range s.Open {
		if o == id {
			from = i
		}
	}
	if from < 0 || to < 0 || to > len(s.Open)-1 || to == from {
		return false
	}
	moved := s.Open[from]
	open := append(s.Open[:from], s.Open[from+1:]...)
	open = append(open[:to], append([]string{moved}, open[to:]...)...)
	s.Open = open
	return true
}

// Close takes a chat out of the dock, choosing another one to show.
func (s *Store) Close(id string) {
	open := s.Open[:0]
	for _, o := range s.Open {
		if o != id {
			open = append(open, o)
		}
	}
	s.Open = open
	if s.Active == id {
		s.Active = ""
		for _, o := range s.Open {
			if s.Find(o) != nil {
				s.Active = o
				break
			}
		}
	}
}

// SetActive makes a chat the one the app shows, adding it to the dock.
func (s *Store) SetActive(id string) {
	if s.Find(id) == nil {
		return
	}
	s.Dock(id)
	s.Active = id
}

// RestoreBuiltins adds the builtin chats the library lacks back to it.
func (s *Store) RestoreBuiltins() int {
	have := map[string]bool{}
	for _, ch := range s.Chats {
		have[normalizeHost(ch.URL)] = true
	}
	added := 0
	for _, ch := range builtinChats() {
		if !have[normalizeHost(ch.URL)] {
			s.Chats = append(s.Chats, ch)
			added++
		}
	}
	return added
}

// hostOf returns the host of a URL without its scheme and www., as a
// fallback name for a chat.
func hostOf(raw string) string {
	u, err := parseURL(raw)
	if err != nil {
		return strings.TrimSpace(raw)
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}

// normalizeHost is hostOf for comparing two chats' URLs.
func normalizeHost(raw string) string {
	u, err := parseURL(raw)
	if err != nil {
		return raw
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// builtinChats are the web chats the app starts with. They are regular
// chats: the user edits, closes or deletes them like the ones they add.
func builtinChats() []*Chat {
	return []*Chat{
		{Name: "ChatGPT", URL: "https://chatgpt.com/", Builtin: true},
		{Name: "Claude", URL: "https://claude.ai/new", Builtin: true},
		{Name: "Gemini", URL: "https://gemini.google.com/app", Builtin: true},
		{Name: "DeepSeek", URL: "https://chat.deepseek.com/", Builtin: true},
		{Name: "豆包", URL: "https://www.doubao.com/chat/", Builtin: true},
		{Name: "Kimi", URL: "https://www.kimi.com/", Builtin: true},
		{Name: "通义千问", URL: "https://chat.qwen.ai/", Builtin: true},
		{Name: "智谱清言", URL: "https://chatglm.cn/main/alltoolsdetail", Builtin: true},
		{Name: "文心一言", URL: "https://yiyan.baidu.com/", Builtin: true},
		{Name: "腾讯元宝", URL: "https://yuanbao.tencent.com/chat/", Builtin: true},
		{Name: "Grok", URL: "https://grok.com/", Builtin: true},
		{Name: "Mistral", URL: "https://chat.mistral.ai/chat", Builtin: true},
		{Name: "Perplexity", URL: "https://www.perplexity.ai/", Builtin: true},
		{Name: "Copilot", URL: "https://copilot.microsoft.com/", Builtin: true},
	}
}
