package launcher

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/nsgo-dev/nsgo/core"

	"my-quickstart-go/internal/branding"
	"my-quickstart-go/internal/libraryui"
	"my-quickstart-go/internal/model"
	"my-quickstart-go/internal/platform"
	"my-quickstart-go/internal/scheduleui"
)

type nativeWindow struct {
	root, surface                          *core.Scope
	window                                 *core.Window
	tray                                   *core.Tray
	route                                  *core.Signal[nativeRoute]
	dialog, content, groups, dialogContent *core.Signal[int]
	search                                 *core.Signal[bool]
	message                                *core.Signal[string]
	quitPending                            bool
	hotkey                                 *core.Signal[string]
	icons                                  map[string]*core.Signal[[]byte]
}

type App struct {
	native           *nativeWindow
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
	icons            map[string]*ui.Bitmap
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

func (a *App) Mount(cx *core.Scope) {
	n := &nativeWindow{root: cx, hotkey: core.NewSignalFunc(cx, a.state.Settings.ToggleHotkey, nil), icons: make(map[string]*core.Signal[[]byte])}
	a.native = n
	n.route = core.NewSignal(cx, a.currentRoute())
	n.dialog = core.NewSignal(cx, a.currentDialog())
	n.content, n.groups, n.dialogContent = core.NewSignal(cx, 0), core.NewSignal(cx, 0), core.NewSignal(cx, 0)
	n.search, n.message = core.NewSignal(cx, a.searchOpen), core.NewSignal(cx, a.message)
	opts := core.WindowOptions{Title: "My Quickstart Go", Width: 960, Height: 620, Hidden: true, Constraints: core.WindowConstraints{MinWidth: 560, MinHeight: 380}}
	if b := a.state.Settings.WindowBounds; a.state.Settings.RememberWindowBounds && b != nil && b.Width >= 560 && b.Height >= 380 {
		for _, display := range cx.Env().Displays() {
			f := display.VisibleFrame
			if b.X+b.Width > int(f.Origin.X) && b.Y+b.Height > int(f.Origin.Y) && b.X < int(f.Origin.X+f.Size.Width) && b.Y < int(f.Origin.Y+f.Size.Height) {
				opts.Frame = core.WindowFrame{Move: true, X: core.Points(b.X), Y: core.Points(b.Y), Width: core.Points(b.Width), Height: core.Points(b.Height)}
				break
			}
		}
	}
	n.window = cx.OpenWindow(opts, func(surface *core.Scope) core.Node {
		n.surface = surface
		a.nativeAppearance()
		surface.Effect(func() { a.captureNativeBounds() })
		return a.nativePage(surface)
	})
	cx.SetMainWindow(n.window)
	cx.OnReopen(a.Show)
	n.window.OnClosing(cx, func(e *core.WindowClosing) {
		if !a.quitting {
			e.Cancel()
			a.hide()
		}
	})
	cx.OnQuitRequest(func(q *core.QuitRequest) { a.requestQuit(q.Defer()) })
	n.tray = cx.NewTray(core.TrayOptions{Icon: branding.PNG, Tooltip: "My Quickstart Go", OnClick: a.Show, Menu: func() []core.MenuItem {
		return []core.MenuItem{
			{Label: a.text("显示 / 隐藏", "Show / Hide"), OnSelect: a.Toggle}, {Label: a.text("设置", "Settings"), OnSelect: func() { a.openSettings(); a.Show() }},
			{Separator: true}, {Label: a.text("退出", "Quit"), OnSelect: func() {
				cx.Post(func() {
					a.requestQuit(func(ok bool) {
						if ok {
							cx.Quit()
						}
					})
				})
			}}}
	}})
	cx.Effect(func() {
		key := strings.TrimSpace(n.hotkey.Get())
		a.registeredHotkey = ""
		if key == "" {
			if a.state.Settings.ToggleHotkey != "" {
				a.state.Settings.ToggleHotkey = ""
				a.changed()
			}
			return
		}
		h := cx.RegisterHotKey(strings.ToLower(key), a.Toggle)
		cx.Effect(func() {
			switch h.State() {
			case core.HotKeyRegistered:
				a.registeredHotkey = key
				if a.state.Settings.ToggleHotkey != key {
					a.state.Settings.ToggleHotkey = key
					a.changed()
				}
			case core.HotKeyTaken, core.HotKeyUnsupported:
				a.report(fmt.Errorf("%s: %s", a.text("快捷键不可用", "Shortcut unavailable"), key))
				// Restore the last accepted binding if the replacement was rejected.
				if saved := a.state.Settings.ToggleHotkey; saved != "" && !strings.EqualFold(saved, key) {
					cx.Post(func() {
						if n.hotkey.GetUntracked() == key {
							n.hotkey.Set(saved)
						}
					})
				}
				a.syncNative()
			}
		})
	})
	if a.library != nil {
		a.library.Dispatch = func(fn func()) {
			n.root.Post(func() {
				fn()
				a.syncNative()
				if (!a.settingsOpen && a.mainTab == 1) || (a.settingsOpen && a.settingsTab == 5) {
					bump(n.content)
					if a.currentDialog() == dialogLibrary {
						bump(n.dialogContent)
					}
				}
			})
		}
	}
	a.schedule.StartWithDispatch(cx.Post)
	if a.saver != nil {
		go func() {
			for {
				select {
				case err := <-a.saver.errors:
					a.post(func() { a.report(err) })
				case <-a.saver.stop:
					return
				}
			}
		}()
	}
	if !a.state.Settings.HideOnStartup || n.tray == nil {
		a.Show()
	}
}

// requestQuit runs on the UI thread. A save can wait for SQLite's busy timeout.
func (a *App) requestQuit(answer func(bool)) {
	n := a.native
	if n.quitPending {
		answer(false)
		return
	}
	if a.schedule.Busy() || (a.library != nil && a.library.Busy) {
		a.report(errors.New(a.text("操作进行中，请稍后退出", "Operation in progress; please wait before quitting")))
		a.Show()
		answer(false)
		return
	}
	a.captureNativeBounds()
	if a.saver != nil && a.saver.dirty() {
		n.quitPending = true
		go func() {
			err := a.saver.flush()
			n.root.Post(func() {
				n.quitPending = false
				if err != nil {
					a.report(err)
					a.Show()
					answer(false)
					return
				}
				// Edits and scheduled launches may have happened during the write.
				a.requestQuit(answer)
			})
		}()
		return
	}
	a.quitting = true
	a.cancelHiddenReturn()
	a.schedule.Stop()
	answer(true)
}

func (a *App) post(fn func()) {
	if n := a.native; n != nil {
		n.root.Post(func() { fn(); a.syncNative() })
		return
	}
	a.window.Update(fn)
}

func (a *App) captureNativeBounds() {
	n := a.native
	if n == nil || n.surface == nil {
		return
	}
	env := n.surface.Env()
	f := env.WindowFrame()
	if env.Minimized() || env.Maximized() || !a.state.Settings.RememberWindowBounds || f.Width < 560 || f.Height < 380 {
		return
	}
	b := model.Bounds{X: int(f.X), Y: int(f.Y), Width: int(f.Width), Height: int(f.Height)}
	if old := a.state.Settings.WindowBounds; old != nil && *old == b {
		return
	}
	a.state.Settings.WindowBounds = &b
	if a.saver != nil {
		a.saver.set(a.state)
	}
}

func New(state model.State, p persistence) *App {
	state.Normalize()
	a := &App{state: state, selected: 0, sidebar: float32(state.Settings.SidebarWidth),
		icons: map[string]*ui.Bitmap{}, iconLoading: map[string]chan struct{}{}, iconSlots: make(chan struct{}, 4)}
	if p != nil {
		a.saver = newSaver(p)
	}
	a.schedule = scheduleui.New(&a.state.Tasks, func() {
		a.queueSave()
		if n := a.native; n != nil && a.mainTab == 2 && !a.settingsOpen {
			bump(n.content)
		}
	}, func() error {
		if a.saver != nil {
			return a.saver.flush()
		}
		return nil
	})
	a.filter()
	return a
}

func (a *App) Show() {
	// Apply an elapsed deadline before preparing the first visible frame, even
	// when the timer callback is still queued on the UI thread.
	a.finishHiddenReturn(a.hiddenReturnGeneration, time.Now())
	a.cancelHiddenReturn()
	if n := a.native; n != nil {
		n.window.Show()
		n.window.Focus()
		a.syncNative()
		return
	}
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
	if n := a.native; n != nil {
		if n.window.Visible() && (n.surface == nil || !n.surface.Env().Minimized()) {
			a.hide()
		} else {
			a.Show()
		}
		return
	}
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
func (a *App) canHide() bool {
	return a.tray != nil || a.native != nil && a.native.tray != nil || a.registeredHotkey != ""
}

func (a *App) hide() {
	if !a.canHide() {
		a.report(errors.New(a.text("没有可用的托盘或全局快捷键，窗口将保持显示。", "No tray or global hotkey is available; the window will stay visible.")))
		return
	}
	if a.warming {
		a.showPending = false
		return
	}
	if n := a.native; n != nil {
		a.captureNativeBounds()
		n.window.Hide()
		a.startHiddenReturn(a.post)
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
	a.changed()
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

func (a *App) queueSave() {
	if a.saver != nil {
		a.saver.set(a.state)
	}
}

func (a *App) changed() {
	a.state.Normalize()
	a.sidebar = float32(a.state.Settings.SidebarWidth)
	a.filter()
	a.queueSave()
	if a.native != nil {
		a.syncNative()
		if a.mainTab == 0 && !a.settingsOpen {
			bump(a.native.groups)
		} else {
			bump(a.native.content)
		}
	}
}
func (a *App) deferAction(fn func()) { a.pending = append(a.pending, fn) }
func (a *App) report(err error) {
	if err == nil {
		return
	}
	log.Print(err)
	a.message = err.Error()
	if a.native != nil {
		a.native.message.Set(a.message)
	}
}
func (a *App) text(zh, en string) string {
	if a.state.Settings.Language == "en" {
		return en
	}
	return zh
}

func (a *App) launch(e model.Entry) {
	if a.busy {
		return
	}
	a.busy = true
	go func() {
		err := platform.Launch(e)
		a.post(func() {
			a.busy = false
			if err != nil {
				a.report(err)
			}
		})
	}()
}

func (a *App) icon(e model.Entry) *ui.Bitmap {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(e.Path)), "command:") {
		return nil
	}
	key := e.Icon + "\x00" + e.Path
	if b, ok := a.icons[key]; ok {
		return b
	}
	if a.iconLoading[key] != nil || a.window == nil {
		return nil
	}
	done := make(chan struct{})
	a.iconLoading[key] = done
	go func() {
		a.iconSlots <- struct{}{}
		defer func() { <-a.iconSlots }()
		path := e.Path
		if e.Icon != "" {
			path = e.Icon
		}
		data, err := platform.Icon(path, 128)
		var b *ui.Bitmap
		if err == nil {
			b, err = ui.DecodeBitmap(data)
		}
		if err != nil {
			log.Printf("icon %s: %v", e.Path, err)
		}
		a.window.Update(func() {
			a.icons[key] = b
			delete(a.iconLoading, key)
			close(done)
		})
	}()
	return nil
}
