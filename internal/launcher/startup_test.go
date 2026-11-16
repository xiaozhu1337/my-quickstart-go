package launcher

import (
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/library"
	"my-quickstart-go/internal/model"
)

func TestStartupOnlyWarmsFirstScreen(t *testing.T) {
	a := fixture()
	group := &a.state.Groups[0]
	for len(group.Apps) < 1000 {
		group.Apps = append(group.Apps, model.NewEntry("App", "app.exe", ""))
	}
	a.filter()
	for _, size := range [][2]int{{560, 380}, {960, 620}, {12000, 12000}} {
		entries := a.startupEntries(size[0], size[1])
		if len(entries) == 0 || len(entries) > 128 || len(entries) >= len(a.results) || entries[0].ID != a.results[0].ID {
			t.Fatalf("startup loaded the wrong range at %v: %d", size, len(entries))
		}
	}
	a.state.Groups[0].Apps = nil
	a.filter()
	if len(a.startupEntries(560, 380)) != 0 {
		t.Fatal("empty groups must not start icon loads")
	}
}

func TestStartupShowAndToggleRequests(t *testing.T) {
	a := fixture()
	a.warming = true
	a.registeredHotkey = "Ctrl+Alt+Space"
	a.Show()
	if !a.showPending || !a.warming {
		t.Fatal("show must wait for preparation")
	}
	a.Toggle()
	if a.showPending {
		t.Fatal("a second shortcut during preparation must cancel the show request")
	}
	a.Toggle()
	if !a.showPending {
		t.Fatal("the shortcut must request the window again")
	}
	a.hide()
	if a.showPending {
		t.Fatal("hide must cancel a pending show")
	}
	a.finishStartup()
	if a.warming || a.showPending {
		t.Fatal("hidden startup must finish without displaying the window")
	}
	a.warming = true
	a.Show()
	a.finishStartup()
	if a.warming || a.showPending {
		t.Fatal("startup must consume its show request exactly once")
	}
	a.warming, a.showPending, a.quitting = true, true, true
	a.finishStartup()
	if !a.warming || !a.showPending {
		t.Fatal("a late startup callback must not continue after quit")
	}
}

func TestIconPreparationWaitsAndHasDeadline(t *testing.T) {
	first, second := make(chan struct{}), make(chan struct{})
	finished := make(chan struct{})
	go func() {
		waitForIcons([]<-chan struct{}{first, second}, time.Second)
		close(finished)
	}()
	close(first)
	select {
	case <-finished:
		t.Fatal("the second icon has not completed")
	default:
	}
	close(second)
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("completed icons did not release preparation")
	}
	at := time.Now()
	waitForIcons([]<-chan struct{}{make(chan struct{}), make(chan struct{})}, 10*time.Millisecond)
	if time.Since(at) > time.Second {
		t.Fatal("one slow target must not indefinitely block showing the window")
	}
	waitForIcons(nil, time.Hour)
}

func TestHiddenReturnDeadline(t *testing.T) {
	for _, page := range []string{"launcher", "library", "settings", "library-settings", "schedule"} {
		for _, seconds := range []float64{3, 10} {
			t.Run(fmt.Sprintf("%s/%g", page, seconds), func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					a := fixture()
					a.state.Settings.ReturnToLauncherSeconds = seconds
					a.mainTab = 0
					if page == "library" || page == "library-settings" {
						a.mainTab = 1
					}
					if page == "schedule" {
						a.mainTab = 2
					}
					a.settingsOpen = page == "settings" || page == "library-settings"
					callbacks := make(chan func(), 2)
					a.startHiddenReturn(func(fn func()) { callbacks <- fn })
					defer a.cancelHiddenReturn()
					if page == "launcher" {
						if a.hiddenReturnTimer != nil {
							t.Fatal("launcher page must not start a timer")
						}
						return
					}
					time.Sleep(time.Duration(seconds)*time.Second - time.Millisecond)
					synctest.Wait()
					if len(callbacks) != 0 {
						t.Fatal("returned before the configured deadline")
					}
					// Repeated hide notifications must not postpone the deadline.
					a.startHiddenReturn(func(fn func()) { callbacks <- fn })
					time.Sleep(time.Millisecond)
					synctest.Wait()
					select {
					case fn := <-callbacks:
						fn()
					default:
						t.Fatal("timer did not dispatch at the configured deadline")
					}
					if a.mainTab != 0 || a.settingsOpen || a.hiddenReturnTimer != nil {
						t.Fatal("expired hide must return to launcher and release the timer")
					}
				})
			})
		}
	}
}

func TestHiddenReturnReopenAndStaleCallbacks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a := fixture()
		a.mainTab, a.settingsOpen = 1, true
		callbacks := make(chan func(), 2)
		dispatch := func(fn func()) { callbacks <- fn }
		a.startHiddenReturn(dispatch)
		time.Sleep(9 * time.Second)
		a.Show()
		if a.mainTab != 1 || !a.settingsOpen || a.hiddenReturnTimer != nil {
			t.Fatal("early reopen must preserve the page and cancel the timer")
		}
		a.startHiddenReturn(dispatch)
		defer a.cancelHiddenReturn()
		time.Sleep(time.Second)
		synctest.Wait()
		if len(callbacks) != 0 {
			t.Fatal("the cancelled timer must not fire during the next hide")
		}
		time.Sleep(9 * time.Second)
		synctest.Wait()
		if len(callbacks) != 1 {
			t.Fatal("the next hide must receive a full new delay")
		}
		stale := <-callbacks
		// Showing after the deadline must work even before the queued callback runs.
		a.Show()
		if a.mainTab != 0 || a.settingsOpen {
			t.Fatal("late reopen must show launcher immediately")
		}
		a.mainTab, a.settingsOpen = 1, true
		a.startHiddenReturn(dispatch)
		stale()
		if a.mainTab != 1 || !a.settingsOpen || a.hiddenReturnTimer == nil {
			t.Fatal("a stale callback must not reset a later hide cycle")
		}
		a.BeforeQuit(&mygo.QuitEvent{})
		time.Sleep(10 * time.Second)
		synctest.Wait()
		if !a.quitting || a.hiddenReturnTimer != nil || len(callbacks) != 0 {
			t.Fatal("quit must cancel pending timers")
		}
		stale()
		if a.mainTab != 1 || !a.settingsOpen {
			t.Fatal("callbacks must not change pages after quit")
		}
	})
}

func TestHiddenReturnClearsLibrarySelection(t *testing.T) {
	root := t.TempDir()
	resources, err := library.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err = resources.SaveMCP("", "selected-resource", "", "command: tool"); err != nil {
		t.Fatal(err)
	}
	a := fixture()
	a.SetLibrary(root)
	a.mainTab = 1
	tt := ui.NewTester(a.view, 960, 620)
	if err = tt.Click("selected-resource"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("1 selected") {
		t.Fatal("resource was not selected")
	}
	a.hiddenReturnDeadline = time.Now().Add(-time.Second)
	a.finishHiddenReturn(a.hiddenReturnGeneration, time.Now())
	if a.mainTab != 0 {
		t.Fatal("expired hide did not return to launcher")
	}
	a.mainTab = 1
	tt.Frame()
	if !tt.HasText("0 selected") {
		t.Fatal("expired hide kept the library selection")
	}
}

func TestHiddenReturnSetting(t *testing.T) {
	for _, language := range []string{"en", "zh-CN"} {
		for _, width := range []int{560, 960} {
			a := fixture()
			a.state.Settings.Language = language
			p := &memoryStore{}
			a.saver = &saver{store: p, wake: make(chan struct{}, 1)}
			a.openSettings()
			a.settingsTab = 2
			tt := ui.NewTester(a.view, width, 620)
			label := a.text("自动返回等待秒数", "Return delay in seconds")
			r, ok := tt.Find(label)
			if !ok {
				t.Fatal("return delay control is missing")
			}
			tt.ClickAt(r.X+20, r.Y+r.H/2)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("25")
			tt.Key(0, ui.KeyEnter)
			if a.state.Settings.ReturnToLauncherSeconds != 25 {
				t.Fatalf("setting did not update: %g, language=%s width=%d", a.state.Settings.ReturnToLauncherSeconds, language, width)
			}
			if err := a.Flush(); err != nil || p.state.Settings.ReturnToLauncherSeconds != 25 {
				t.Fatalf("setting did not persist: %v", err)
			}
		}
	}
}

func TestHiddenReturnResetsScheduledDialogs(t *testing.T) {
	for _, action := range []string{"Add task", "Edit", "Delete"} {
		t.Run(action, func(t *testing.T) {
			a := fixture()
			a.mainTab = 2
			saved := model.ScheduledTask{ID: "saved", Name: "Saved task", Command: "backup.exe --daily", Time: "09:00", Enabled: true}
			a.state.Tasks = []model.ScheduledTask{saved}
			tt := ui.NewTester(a.view, 960, 620)
			if err := tt.Click(action); err != nil {
				t.Fatal(err)
			}
			if !a.schedule.ModalOpen() {
				t.Fatal("scheduled dialog did not open")
			}
			if action == "Add task" {
				if err := tt.Click("Task name"); err != nil {
					t.Fatal(err)
				}
				tt.Type("Unsaved draft")
			}
			a.hiddenReturnDeadline = time.Now().Add(time.Second)
			a.Show()
			if a.mainTab != 2 || !a.schedule.ModalOpen() {
				t.Fatal("early reopen must retain the dialog")
			}
			a.hiddenReturnDeadline = time.Now().Add(-time.Second)
			a.Show()
			if a.mainTab != 0 || a.schedule.ModalOpen() {
				t.Fatal("elapsed hide must return to launcher and close scheduled dialogs")
			}
			tt.Frame()
			if !tt.HasText("Alpha App") || tt.HasText("New scheduled task") || tt.HasText("Delete scheduled task") {
				t.Fatal("scheduled modal remained visible over launcher")
			}
			if err := tt.Click("Scheduled tasks"); err != nil {
				t.Fatal(err)
			}
			if a.schedule.ModalOpen() || !tt.HasText("Saved task") || len(a.state.Tasks) != 1 || a.state.Tasks[0] != saved {
				t.Fatal("returning to schedules must show the saved list without stale dialogs")
			}
			if err := tt.Click("Add task"); err != nil {
				t.Fatal(err)
			}
			if tt.HasText("Unsaved draft") {
				t.Fatal("discarded draft returned in the next editor")
			}
		})
	}
}
