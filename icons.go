package main

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	xdraw "golang.org/x/image/draw"
)

// iconStore keeps the chats' icons as files beside chats.json, and the
// pictures the GPU draws of them, decoded once.
type iconStore struct {
	mu    sync.Mutex
	dir   string
	cache map[string]ui.ImageSource
	fails map[string]bool
}

// newIconStore makes the icons directory of a data directory.
func newIconStore(dir string) *iconStore {
	_ = os.MkdirAll(dir, 0o755)
	return &iconStore{dir: dir, cache: map[string]ui.ImageSource{}, fails: map[string]bool{}}
}

// get returns the picture of an icon file, nil when it has none the app
// can draw.
func (s *iconStore) get(name string) ui.ImageSource {
	if name == "" {
		return nil
	}
	s.mu.Lock()
	pic, ok := s.cache[name]
	failed := s.fails[name]
	s.mu.Unlock()
	if ok {
		return pic
	}
	if failed {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(s.dir, name))
	if err != nil {
		s.mu.Lock()
		s.fails[name] = true
		s.mu.Unlock()
		return nil
	}
	pic, _, err = decodeIcon(data)
	if err != nil {
		s.mu.Lock()
		s.fails[name] = true
		s.mu.Unlock()
		return nil
	}
	s.mu.Lock()
	s.cache[name] = pic
	s.mu.Unlock()
	return pic
}

// put writes icon data as a new icon file of the chat, and returns its name
// and picture. Run it on the main thread, as uis are made there.
func (s *iconStore) put(chatID string, data []byte) (string, ui.ImageSource, error) {
	pic, ext, err := decodeIcon(data)
	if err != nil {
		return "", nil, err
	}
	name := chatID + "-" + strconv.FormatInt(time.Now().UnixNano(), 36) + "." + ext
	if err := os.WriteFile(filepath.Join(s.dir, name), data, 0o644); err != nil {
		return "", nil, err
	}
	s.mu.Lock()
	s.cache[name] = pic
	s.mu.Unlock()
	return name, pic, nil
}

// drop forgets an icon, and removes its file.
func (s *iconStore) drop(name string) {
	if name == "" {
		return
	}
	s.mu.Lock()
	delete(s.cache, name)
	delete(s.fails, name)
	s.mu.Unlock()
	_ = os.Remove(filepath.Join(s.dir, name))
}

// decodeIcon decodes the formats sites give their icons in: PNG, JPEG,
// GIF, WebP, BMP, SVG, and an ICO that holds a PNG.
func decodeIcon(data []byte) (ui.ImageSource, string, error) {
	if len(data) == 0 {
		return nil, "", errors.New("图标为空")
	}
	if img, format, err := image.Decode(bytes.NewReader(data)); err == nil {
		return ui.NewBitmap(shrink(img, 256)), shortExt(format), nil
	}
	if pic, err := ui.DecodeImage(data); err == nil {
		return pic, sniffExt(data), nil
	}
	// An ICO holds PNG or BMP pictures; take the first PNG it has.
	if i := bytes.Index(data, []byte("\x89PNG\r\n\x1a\n")); i >= 0 {
		if img, err := png.Decode(bytes.NewReader(data[i:])); err == nil {
			return ui.NewBitmap(shrink(img, 256)), "ico", nil
		}
	}
	return nil, "", errors.New("不支持的图标格式")
}

// shrink scales a picture down to at most px wide and high, for the dock
// that shows it at 32 DIPs.
func shrink(img image.Image, px int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= px && h <= px {
		return img
	}
	scale := float64(px) / float64(max(w, h))
	dst := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Over, nil)
	return dst
}

// sniffExt names a picture image.Decode does not know by its contents.
func sniffExt(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte("BM")):
		return "bmp"
	case len(data) > 12 && bytes.HasPrefix(data, []byte("RIFF")) && string(data[8:12]) == "WEBP":
		return "webp"
	case bytes.Contains(data[:min(len(data), 512)], []byte("<svg")):
		return "svg"
	default:
		return "img"
	}
}

// shortExt names a format image.Decode reports, as a file extension.
func shortExt(format string) string {
	if format == "jpeg" {
		return "jpg"
	}
	if format == "" {
		return "bin"
	}
	return strings.ToLower(format)
}
