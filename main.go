// chatBox aggregates web chats in one window: each chat is a page of the
// system's web view in the dock at the left, which switches between them,
// and each one takes its own CSS and JS.
package main

import (
	"log"
	"os"

	"github.com/egoist/mygo"
)

// version is the app's version, shown by mygo build.
const version = "0.1.1"

func main() {
	// CHATBOX_DEBUG_LOG=<path> keeps the app's log in a file of its own,
	// which a check can read while the app runs, as a GUI app's run is
	// otherwise buffered by whatever pipes it.
	if path := envVar("DEBUG_LOG"); path != "" {
		f, err := os.Create(path)
		if err == nil {
			log.SetOutput(f)
		}
	}
	mygo.App.SetName("chatBox")
	mygo.App.SetVersion(version)

	a, err := newApp()
	if err != nil {
		log.Fatalf("myChatBox: %v", err)
	}
	mygo.App.WhenReady(a.start)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
