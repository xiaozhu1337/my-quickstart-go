package launcher

import (
	"log"
	"math"
	"time"

	"my-quickstart-go/internal/model"
	"my-quickstart-go/internal/platform"
)

func (a *App) startupEntries(width, height int) []model.Entry {
	s := a.state.Settings
	columns := max(1, int((float64(width)-s.SidebarWidth-30)/(s.CardWidth+10)))
	rows := max(1, int(math.Ceil(float64(height)/(s.CardHeight+10)))+1)
	// shortcut: warm at most 128 first-screen cards; larger grids keep loading on demand.
	return a.results[:min(len(a.results), columns*rows, 128)]
}

func (a *App) prepareStartup() {
	bounds := a.window.Bounds()
	var pending []<-chan struct{}
	for _, e := range a.startupEntries(bounds.Width, bounds.Height) {
		a.icon(e)
		if done := a.iconLoading[e.Icon+"\x00"+e.Path]; done != nil {
			pending = append(pending, done)
		}
	}
	go func() {
		// A missing or remote target must not prevent the window from becoming usable.
		waitForIcons(pending, 500*time.Millisecond)
		a.window.Update(a.finishStartup)
	}()
}

func waitForIcons(pending []<-chan struct{}, budget time.Duration) {
	timer := time.NewTimer(budget)
	defer timer.Stop()
	for _, done := range pending {
		select {
		case <-done:
		case <-timer.C:
			return
		}
	}
}

func (a *App) finishStartup() {
	if a.quitting {
		return
	}
	show := a.showPending
	a.warming, a.showPending = false, false
	if show {
		a.Show()
	} else {
		a.prepareWindow()
	}
}

func (a *App) prepareWindow() {
	if a.window == nil || a.window.IsDestroyed() || a.window.IsMinimized() {
		return
	}
	if err := platform.PrepareWindow(a.window); err != nil {
		log.Printf("prepare window: %v", err)
	}
}
