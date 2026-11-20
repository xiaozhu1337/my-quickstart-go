package launcher

import (
	"container/list"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/branding"
	"my-quickstart-go/internal/libraryui"
	"my-quickstart-go/internal/model"
	"my-quickstart-go/internal/platform"
	"my-quickstart-go/internal/scheduleui"
	"my-quickstart-go/internal/store"
)

type App struct {
	mainTab          int
	library          *libraryui.Panel
	schedule         *scheduleui.Panel
	state            model.State
	saver            *saver
	window           *mygo.Window
	tray             *mygo.Tray
	query            string
	searchOpen       bool
	filteredQuery    string
	searchNames      map[string][]string
	browseGroupID    string
	dragPreviewID    string
	dragBeforeID     string
	matches          map[string]int
	results          []model.Entry
	selected         int
	grid             ui.GridState
	groups           ui.ListState
	search           ui.Handle
	sidebar          float32
	icons            map[string]*list.Element
	iconOrder        list.List
	iconBytes        int
	iconLoading      map[string]chan struct{}
	iconSlots        chan struct{}
	warming          bool
	showPending      bool
	settingsOpen     bool
	settingsTab      int
	hotkey           string
	registeredHotkey string
	editorOpen       bool
	editID           string
	editGroup        string
	draft            model.Entry
	envText          string
	command          bool
	renameOpen       bool
	renameID         string
	renameText       string
	addOpen          bool
	uwp              []model.Entry
	uwpQuery         string
	uwpLoading       bool
	uwpError         string
	uwpList          ui.ListState
	message          string
	pending          []func()
	busy             bool
	quitting         bool
	fonts            []string

	hiddenReturnTimer      *time.Timer
	hiddenReturnDeadline   time.Time
	hiddenReturnGeneration uint64
}

func (a *App) SetLibrary(root string) { a.library = libraryui.New(root) }

func New(state model.State, p persistence) *App {
	state.Normalize()
	a := &App{state: state, selected: 0, sidebar: float32(state.Settings.SidebarWidth),
		icons: map[string]*list.Element{}, iconLoading: map[string]chan struct{}{}, iconSlots: make(chan struct{}, 4)}
	if p != nil {
		a.saver = newSaver(p, state)
	}
	a.schedule = scheduleui.New(&a.state.Tasks, func() { a.persist(store.Tasks) }, func() error {
		if a.saver != nil {
			return a.saver.flush()
		}
		return nil
	})
	a.filter()
	return a
}

func (a *App) Ready() {
	a.warming = true
	syncNativeTheme(a.state.Settings.Theme)
	if err := mygo.App.Dock.SetIcon(branding.PNG); err != nil {
		a.report(fmt.Errorf("%s: %w", a.text("无法设置应用图标", "Cannot set application icon"), err))
	}
	opts := mygo.WindowOptions{Title: "My Quickstart Go", Width: 960, Height: 620, MinWidth: 560, MinHeight: 380,
		Hidden: true, AlwaysOnTop: a.state.Settings.AlwaysOnTop, BackgroundColor: backgroundColor(a.state.Settings.Theme), Content: ui.View(a.view)}
	if b := a.state.Settings.WindowBounds; a.state.Settings.RememberWindowBounds && b != nil && b.Width >= 560 && b.Height >= 380 {
		r := mygo.Rectangle{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height}
		for _, d := range mygo.Screen.Displays() {
			w := d.WorkArea
			if r.X+r.Width > w.X && r.Y+r.Height > w.Y && r.X < w.X+w.Width && r.Y < w.Y+w.Height {
				opts.X, opts.Y, opts.Width, opts.Height = r.X, r.Y, r.Width, r.Height
				break
			}
		}
	}
	a.window = mygo.NewWindow(opts)
	if a.library != nil {
		a.library.Window = a.window
	}
	if err := a.window.SetIcon(branding.PNG); err != nil {
		a.report(fmt.Errorf("%s: %w", a.text("无法设置窗口图标", "Cannot set window icon"), err))
	}
	a.window.OnClose(a.onClose)
	a.window.OnHide(func() { a.startHiddenReturn(a.window.Update) })
	a.window.OnShow(a.cancelHiddenReturn)
	a.window.OnResize(a.captureBounds)
	a.window.OnMove(a.captureBounds)
	a.schedule.Start(a.window)
	mygo.App.SetLanguages(a.state.Settings.Language)
	if err := a.applyHotkey(a.state.Settings.ToggleHotkey); err != nil {
		a.report(err)
	}
	var err error
	a.tray, err = mygo.NewTray(mygo.TrayOptions{Icon: branding.PNG, ToolTip: "My Quickstart Go"})
	if err != nil {
		a.report(err)
	} else {
		a.rebuildTray()
		a.tray.OnClick(a.Show)
	}
	if a.saver != nil {
		go func() {
			for {
				select {
				case err := <-a.saver.errors:
					a.window.Update(func() { a.report(err) })
				case <-a.saver.stop:
					return
				}
			}
		}()
	}
	go func() { fonts := platform.Fonts(); a.window.Update(func() { a.fonts = fonts }) }()
	if !a.state.Settings.HideOnStartup || a.tray == nil {
		a.Show()
	}
	a.prepareStartup()
}

func (a *App) Show() {
	// Apply an elapsed deadline before preparing the first visible frame, even
	// when the timer callback is still queued on the UI thread.
	a.finishHiddenReturn(a.hiddenReturnGeneration, time.Now())
	a.cancelHiddenReturn()
	if a.warming {
		a.showPending = true
		return
	}
	if a.window != nil {
		a.prepareWindow()
		a.window.Show()
		if a.window.IsMinimized() {
			a.window.Restore()
		}
		a.window.Focus()
		if a.searchOpen && a.mainTab == 0 {
			a.search.Focus()
		}
	}
}
func (a *App) Toggle() {
	if a.warming {
		a.showPending = !a.showPending
		return
	}
	if a.window == nil {
		return
	}
	if a.window.IsMinimized() {
		a.Show()
	} else if a.window.IsVisible() {
		a.hide()
	} else {
		a.Show()
	}
}
func (a *App) canHide() bool { return a.tray != nil || a.registeredHotkey != "" }

func (a *App) hide() {
	if !a.canHide() {
		a.report(errors.New(a.text("没有可用的托盘或全局快捷键，窗口将保持显示。", "No tray or global hotkey is available; the window will stay visible.")))
		return
	}
	if a.warming {
		a.showPending = false
		return
	}
	a.captureBounds()
	if a.window != nil {
		a.window.Hide()
	}
}

func (a *App) captureBounds() {
	if a.window == nil || !a.state.Settings.RememberWindowBounds || a.window.IsMinimized() || a.window.IsMaximized() {
		return
	}
	r := a.window.Bounds()
	if r.Width < 560 || r.Height < 380 {
		return
	}
	b := model.Bounds{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}
	if old := a.state.Settings.WindowBounds; old != nil && *old == b {
		return
	}
	a.state.Settings.WindowBounds = &b
	a.settingsChanged()
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

func (a *App) Flush() error {
	if a.saver == nil {
		return nil
	}
	err := a.saver.flush()
	if err != nil {
		a.report(err)
		a.Show()
		if a.window != nil {
			a.window.Invalidate()
		}
	}
	return err
}
func (a *App) Close() {
	a.schedule.Stop()
	a.cancelHiddenReturn()
	if a.saver != nil {
		close(a.saver.stop)
		<-a.saver.done
	}
	if a.tray != nil {
		a.tray.Destroy()
	}
}

func (a *App) changed() {
	a.state.Normalize()
	a.sidebar = float32(a.state.Settings.SidebarWidth)
	a.searchNames = nil
	a.filter()
	a.pruneIcons()
	a.persist(store.Groups | store.ActiveGroup)
}

func (a *App) persist(fields store.Fields) {
	if a.saver != nil {
		a.saver.update(a.state, fields)
	}
}

func (a *App) settingsChanged() {
	a.state.Settings.Normalize()
	a.sidebar = float32(a.state.Settings.SidebarWidth)
	a.persist(store.Settings)
}
func (a *App) deferAction(fn func()) { a.pending = append(a.pending, fn) }
func (a *App) report(err error) {
	if err == nil {
		return
	}
	log.Print(err)
	a.message = err.Error()
}
func (a *App) text(zh, en string) string {
	if a.state.Settings.Language == "en" {
		return en
	}
	return zh
}

func (a *App) applyHotkey(hotkey string) error {
	hotkey = strings.TrimSpace(hotkey)
	old := a.registeredHotkey
	if strings.EqualFold(hotkey, old) {
		return nil
	}
	if hotkey != "" {
		if err := mygo.GlobalShortcut.Register(hotkey, a.Toggle); err != nil {
			return fmt.Errorf("%s: %w", a.text("快捷键不可用", "Hotkey unavailable"), err)
		}
	}
	if old != "" {
		mygo.GlobalShortcut.Unregister(old)
	}
	a.registeredHotkey = hotkey
	return nil
}

func (a *App) rebuildTray() {
	if a.tray == nil {
		return
	}
	a.tray.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Label: a.text("显示 / 隐藏", "Show / Hide"), Click: func(*mygo.MenuItem, *mygo.Window) { a.Toggle() }},
		{Label: a.text("设置", "Settings"), Click: func(*mygo.MenuItem, *mygo.Window) { a.Show(); a.openSettings(); a.window.Invalidate() }},
		mygo.Separator(), {Label: a.text("退出", "Quit"), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	}))
}

func (a *App) launch(e model.Entry) {
	if a.busy {
		return
	}
	a.busy = true
	go func() {
		err := platform.Launch(e)
		a.window.Update(func() {
			a.busy = false
			if err != nil {
				a.report(err)
			}
		})
	}()
}

func iconKey(e model.Entry) string {
	path := strings.TrimSpace(e.Path)
	if len(path) >= 8 && strings.EqualFold(path[:8], "command:") {
		return ""
	}
	if e.Icon != "" {
		return e.Icon
	}
	return e.Path
}

type cachedIcon struct {
	key    string
	bitmap *ui.Bitmap
	bytes  int
}

func (a *App) icon(e model.Entry) *ui.Bitmap {
	key := iconKey(e)
	if key == "" {
		return nil
	}
	if cached := a.icons[key]; cached != nil {
		a.iconOrder.MoveToFront(cached)
		return cached.Value.(cachedIcon).bitmap
	}
	if a.iconLoading[key] != nil || a.window == nil {
		return nil
	}
	done := make(chan struct{})
	a.iconLoading[key] = done
	go func() {
		a.iconSlots <- struct{}{}
		b, err := platform.IconBitmap(key, 128)
		<-a.iconSlots
		if err != nil {
			log.Printf("icon %s: %v", e.Path, err)
		}
		a.window.Update(func() {
			defer close(done)
			if a.iconLoading[key] != done {
				return
			}
			delete(a.iconLoading, key)
			a.cacheIcon(key, b)
		})
	}()
	return nil
}

func (a *App) cacheIcon(key string, b *ui.Bitmap) {
	size := len(key) + 128
	if b != nil {
		w, h := b.Size()
		size += w * h * 4
	}
	a.icons[key] = a.iconOrder.PushFront(cachedIcon{key, b, size})
	a.iconBytes += size
	// Keep the newest item even if its data URL alone exceeds the budget.
	for a.iconOrder.Len() > 1 && a.iconBytes > 32<<20 {
		a.evictIcon(a.iconOrder.Back())
	}
}

func (a *App) evictIcon(el *list.Element) {
	item := a.iconOrder.Remove(el).(cachedIcon)
	delete(a.icons, item.key)
	a.iconBytes -= item.bytes
}

func (a *App) pruneIcons() {
	if len(a.icons)+len(a.iconLoading) == 0 {
		return
	}
	used := make(map[string]bool)
	for _, g := range a.state.Groups {
		for _, e := range g.Apps {
			used[iconKey(e)] = true
		}
	}
	for key, el := range a.icons {
		if !used[key] {
			a.evictIcon(el)
		}
	}
	for key := range a.iconLoading {
		if !used[key] {
			delete(a.iconLoading, key)
		}
	}
}
