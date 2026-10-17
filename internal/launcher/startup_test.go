package launcher

import (
	"testing"
	"time"

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

func TestWindowBackgroundMatchesEveryTheme(t *testing.T) {
	want := map[string]string{"dark": "#111720", "light": "#f2f5fa", "midnight": "#0d1429", "forest": "#111d1b", "rose": "#faf2f3"}
	for id, color := range want {
		if got := backgroundColor(id); got != color {
			t.Errorf("%s: native background %s, want %s", id, got, color)
		}
	}
}
