package launcher

import (
	"errors"
	"testing"

	"github.com/egoist/mygo"
	"my-quickstart-go/internal/model"
)

type memoryStore struct {
	state  model.State
	writes int
	fail   bool
}

func (m *memoryStore) Path() string { return "memory" }
func (m *memoryStore) Save(s model.State) error {
	if m.fail {
		return errors.New("write failed")
	}
	m.state = s
	m.writes++
	return nil
}

func TestCloseHidesAndQuitFlushesBeforeClosing(t *testing.T) {
	p := &memoryStore{fail: true}
	a := New(model.Default(), nil)
	a.saver = &saver{store: p, wake: make(chan struct{}, 1)}
	a.saver.set(a.state)
	closeEvent := &mygo.CloseEvent{}
	a.onClose(closeEvent)
	if !closeEvent.DefaultPrevented() {
		t.Fatal("ordinary close must hide")
	}
	quitEvent := &mygo.QuitEvent{}
	a.BeforeQuit(quitEvent)
	if !quitEvent.DefaultPrevented() || a.quitting {
		t.Fatal("save failure must cancel quit before windows close")
	}
	p.fail = false
	quitEvent = &mygo.QuitEvent{}
	a.BeforeQuit(quitEvent)
	if quitEvent.DefaultPrevented() || !a.quitting || p.writes != 1 {
		t.Fatal("quit must flush pending state")
	}
	closeEvent = &mygo.CloseEvent{}
	a.onClose(closeEvent)
	if closeEvent.DefaultPrevented() {
		t.Fatal("explicit quit must allow the window to close")
	}
}
func TestHideNeedsRecallEntry(t *testing.T) {
	for _, tray := range []bool{false, true} {
		for _, hotkey := range []string{"", "Ctrl+Space"} {
			a := New(model.Default(), nil)
			if tray {
				a.tray = &mygo.Tray{}
			}
			a.registeredHotkey = hotkey
			if a.canHide() != (tray || hotkey != "") {
				t.Fatal("hiding requires a tray or registered hotkey")
			}
			a.hide()
			if !a.canHide() && a.message == "" {
				t.Fatal("unavailable hiding must explain how the window remains reachable")
			}
		}
	}
}

func TestSaverSnapshotsAndRetries(t *testing.T) {
	p := &memoryStore{}
	s := &saver{store: p, wake: make(chan struct{}, 1)}
	state := model.Default()
	state.Groups[0].Apps = []model.Entry{model.NewEntry("App", "app.exe", "")}
	state.Groups[0].Apps[0].EnvVars = []string{"KEY=old"}
	state.Settings.WindowBounds = &model.Bounds{Width: 800, Height: 600}
	s.set(state)
	state.Groups[0].Apps[0].Name = "Modified"
	state.Groups[0].Apps[0].EnvVars[0] = "KEY=new"
	state.Settings.WindowBounds.Width = 900
	p.fail = true
	if err := s.flush(); err == nil {
		t.Fatal("failed save must be reported")
	}
	p.fail = false
	if err := s.flush(); err != nil {
		t.Fatal(err)
	}
	if p.state.Groups[0].Apps[0].Name != "App" || p.state.Groups[0].Apps[0].EnvVars[0] != "KEY=old" || p.state.Settings.WindowBounds.Width != 800 {
		t.Fatal("save snapshot aliases mutable UI state")
	}
	if err := s.flush(); err != nil {
		t.Fatal(err)
	}
	if p.writes != 1 {
		t.Fatal("an unchanged revision must not write again")
	}
	s.set(state)
	if err := s.flush(); err != nil {
		t.Fatal(err)
	}
	if p.state.Groups[0].Apps[0].Name != "Modified" || p.writes != 2 {
		t.Fatal("latest state was not saved")
	}
}
