//go:build !windows

package platform

import "github.com/egoist/mygo"

// PrepareWindow warms native content layout and text without showing the window.
func PrepareWindow(w *mygo.Window) error {
	// shortcut: CapturePage warms CPU content; the desktop backend prepares its GPU when it maps the window.
	_, err := w.CapturePage()
	return err
}
