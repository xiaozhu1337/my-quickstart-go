package launcher

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/model"
	"my-quickstart-go/internal/platform"
)

type App struct {
	state            model.State
	saver            *saver
	window           *mygo.Window
	tray             *mygo.Tray
	query            string
	searchOpen       bool
	matches          map[string]int
	results          []model.Entry
	selected         int
	grid             ui.GridState
	groups           ui.ListState
	search           ui.Handle
	sidebar          float32
	icons            map[string]*ui.Bitmap
	iconLoading      map[string]bool
	iconSlots        chan struct{}
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
}

func New(state model.State, p persistence) *App {
	state.Normalize()
	a := &App{state: state, selected: 0, sidebar: float32(state.Settings.SidebarWidth),
		icons: map[string]*ui.Bitmap{}, iconLoading: map[string]bool{}, iconSlots: make(chan struct{}, 4)}
	if p != nil {
		a.saver = newSaver(p)
	}
	a.filter()
	return a
}

func (a *App) Ready() {
	syncNativeTheme(a.state.Settings.Theme)
	opts := mygo.WindowOptions{Title: "My Quickstart Go", Width: 960, Height: 620, MinWidth: 560, MinHeight: 380,
		Hidden: true, AlwaysOnTop: a.state.Settings.AlwaysOnTop, Content: ui.View(a.view)}
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
	a.window.OnClose(a.onClose)
	a.window.OnResize(a.captureBounds)
	a.window.OnMove(a.captureBounds)
	mygo.App.SetLanguages(a.state.Settings.Language)
	if err := a.applyHotkey(a.state.Settings.ToggleHotkey); err != nil {
		a.report(err)
	}
	var err error
	a.tray, err = mygo.NewTray(mygo.TrayOptions{Icon: trayIcon(), ToolTip: "My Quickstart Go"})
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
}

func (a *App) Show() {
	if a.window != nil {
		a.window.Show()
		if a.window.IsMinimized() {
			a.window.Restore()
		}
		a.window.Focus()
		if a.searchOpen {
			a.search.Focus()
		}
	}
}
func (a *App) Toggle() {
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

func (a *App) onClose(e *mygo.CloseEvent) {
	if !a.quitting {
		e.PreventDefault()
		a.hide()
	}
}

func (a *App) BeforeQuit(e *mygo.QuitEvent) {
	a.captureBounds()
	if a.Flush() != nil {
		e.PreventDefault()
		return
	}
	a.quitting = true
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
	a.filter()
	if a.saver != nil {
		a.saver.set(a.state)
	}
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

func (a *App) icon(e model.Entry) *ui.Bitmap {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(e.Path)), "command:") {
		return nil
	}
	key := e.Icon + "\x00" + e.Path
	if b, ok := a.icons[key]; ok {
		return b
	}
	if a.iconLoading[key] || a.window == nil {
		return nil
	}
	a.iconLoading[key] = true
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
		a.window.Update(func() { a.icons[key] = b; delete(a.iconLoading, key) })
	}()
	return nil
}

func trayIcon() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 4; y < 28; y++ {
		for x := 4; x < 28; x++ {
			if x < 14 || x >= 18 {
				if y < 14 || y >= 18 {
					img.Set(x, y, color.NRGBA{R: 79, G: 124, B: 255, A: 255})
				}
			}
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		log.Print(err)
	}
	return b.Bytes()
}
