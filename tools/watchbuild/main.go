// watchbuild rebuilds the app's bundle whenever a source file changes, so
// that build/darwin-arm64/chatBox.app is always the code in the tree.
//
//	go run ./tools/watchbuild
//
// It polls the files' times rather than asking the system for events, which
// needs no dependency and catches every editor's way of saving. A change
// rebuilds after a short quiet, so that a save in progress is not built.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// watched are the extensions a change of which asks for a build.
var watched = map[string]bool{
	".go":   true,
	".json": true, // mygo.json, the icons' data
	".png":  true, // resources/icon.png
	".svg":  true,
	".mod":  true,
	".sum":  true,
}

// skipped are the directories no change of which asks for a build: the
// build's own output and the caches.
var skipped = map[string]bool{
	".cache":   true,
	"build":    true,
	".git":     true,
	".devdata": true,
	".mygo":    true,
}

func main() {
	root, err := moduleRoot()
	if err != nil {
		log.Fatal(err)
	}
	quiet := time.Second
	if v := os.Getenv("WATCHBUILD_QUIET"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			quiet = d
		}
	}
	log.Printf("watching %s; a change runs make build", root)

	stamp, files := scan(root)
	log.Printf("%d files under watch", files)

	for {
		time.Sleep(quiet / 4)
		next, _ := scan(root)
		if next == stamp {
			continue
		}
		// A save in progress writes more than once: wait for the tree to
		// rest before building it.
		for {
			time.Sleep(quiet)
			settled, _ := scan(root)
			if settled == next {
				break
			}
			next = settled
		}
		stamp = next
		build(root)
		// The build writes nothing under watch, but the tree may have
		// changed again while it ran: scan once more.
		stamp, _ = scan(root)
	}
}

// moduleRoot finds the directory of go.mod, from the working directory up.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above %s", dir)
		}
		dir = parent
	}
}

// scan sums the names, sizes and times of the files under watch, and counts
// them: the sum changes when any of them does.
func scan(root string) (string, int) {
	var b strings.Builder
	n := 0
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipped[d.Name()] || path != root && d.Name() == "." {
				return filepath.SkipDir
			}
			return nil
		}
		if !watched[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		fmt.Fprintf(&b, "%s|%d|%d\n", path, info.Size(), info.ModTime().UnixNano())
		n++
		return nil
	})
	return b.String(), n
}

// build runs the app's build, and says how it went.
func build(root string) {
	start := time.Now()
	cmd := exec.Command("make", "build")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start).Round(10 * time.Millisecond)
	if err != nil {
		log.Printf("build failed after %s:\n%s", elapsed, lastLines(string(out), 20))
		return
	}
	// mygo build says where it put the app; keep the tail of it.
	log.Printf("built in %s\n%s", elapsed, lastLines(string(out), 4))
}

// lastLines is the last n lines of s, which logs of builds read better as.
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := range lines {
		lines[i] = "  " + strings.TrimSpace(lines[i])
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	var out []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
