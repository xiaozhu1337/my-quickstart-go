package launcher

import (
	"errors"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/nsgotest"

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

type gatedStore struct {
	started chan model.State
	release chan error
}

func (s *gatedStore) Path() string { return "memory" }

func (s *gatedStore) Save(state model.State) error {
	s.started <- state
	return <-s.release
}

func TestNativeQuitWhileSaving(t *testing.T) {
	state := model.Default()
	state.Settings.RememberWindowBounds = false
	a := New(state, nil)
	app := nsgotest.StartApp(t, a.Mount)
	defer a.Close()
	p := &gatedStore{make(chan model.State, 3), make(chan error, 3)}
	defer close(p.release)
	// Control the pending revisions without the background autosave timer.
	a.saver = &saver{store: p, wake: make(chan struct{}, 1)}
	defer func() { a.saver = nil }()
	a.queueSave()
	answered, quit := false, false
	answer := func(ok bool) { answered, quit = true, ok }
	a.requestQuit(answer)
	app.WaitUntil(time.Second, func() bool { return len(p.started) == 1 })
	if answered {
		t.Fatal("quit answered before the save completed")
	}
	a.addGroup()
	app.Settle()
	p.release <- errors.New("write denied")
	app.WaitUntil(time.Second, func() bool { return answered })
	if quit || a.quitting || a.message != "write denied" {
		t.Fatal("failed write must keep the window open and report the error")
	}
	answered = false
	a.requestQuit(answer)
	app.WaitUntil(time.Second, func() bool { return len(p.started) == 2 })
	a.addGroup()
	p.release <- nil
	app.WaitUntil(time.Second, func() bool { return len(p.started) == 3 })
	if answered {
		t.Fatal("quit ignored an edit made during the save")
	}
	<-p.started
	<-p.started
	if latest := <-p.started; latest.ActiveGroupID != a.state.ActiveGroupID {
		t.Fatal("the final save omitted the latest group")
	}
	p.release <- nil
	app.WaitUntil(time.Second, func() bool { return answered })
	if !quit || !a.quitting || a.saver.dirty() {
		t.Fatal("quit was not accepted after the final save")
	}
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
	state.Tasks = []model.ScheduledTask{{ID: "task", Name: "Original task", Path: "app.exe", Time: "09:00"}}
	s.set(state)
	state.Tasks[0].Name = "Changed task"
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
	if p.state.Tasks[0].Name != "Original task" {
		t.Fatal("save snapshot aliases mutable scheduled tasks")
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

func (a *App) onClose(e *mygo.CloseEvent) {
	if !a.quitting {
		e.PreventDefault()
		a.hide()
	}
}

func (a *App) BeforeQuit(e *mygo.QuitEvent) {
	if a.schedule.Busy() {
		e.PreventDefault()
		a.report(errors.New(a.text("定时任务正在启动，请稍后再退出", "A scheduled task is starting; please wait before quitting")))
		return
	}
	if a.library != nil && a.library.Busy {
		e.PreventDefault()
		a.report(errors.New(a.text("资源库正在保存，请完成后再退出", "Library operation in progress; please wait before quitting")))
		return
	}
	a.captureBounds()
	if a.Flush() != nil {
		e.PreventDefault()
		return
	}
	a.quitting = true
	a.schedule.Stop()
	a.cancelHiddenReturn()
}
