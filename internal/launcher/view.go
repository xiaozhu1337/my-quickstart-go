package launcher

import (
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	nsw "github.com/nsgo-dev/nsgo/ui"

	"my-quickstart-go/internal/branding"
	"my-quickstart-go/internal/model"
	w "my-quickstart-go/internal/nativeui"
	"my-quickstart-go/internal/platform"
)

type nativeRoute struct {
	tab                                 int
	settings, hideLibrary, hideSchedule bool
	language, theme, font               string
	fontSize                            float64
}

const (
	dialogNone = iota
	dialogLauncher
	dialogLibrary
	dialogSchedule
)

func (a *App) currentRoute() nativeRoute {
	s := a.state.Settings
	if a.mainTab == 1 && s.HideLibraryTab || a.mainTab == 2 && s.HideScheduleTab {
		a.mainTab = 0
	}
	return nativeRoute{a.mainTab, a.settingsOpen, s.HideLibraryTab, s.HideScheduleTab, s.Language, s.Theme, s.FontFamily, s.FontSize}
}

func (a *App) currentDialog() int {
	switch {
	case a.settingsOpen:
		return dialogNone
	case a.mainTab == 0 && (a.editorOpen || a.renameOpen || a.addOpen):
		return dialogLauncher
	case a.mainTab == 1 && a.library != nil && a.library.ModalOpen():
		return dialogLibrary
	case a.mainTab == 2 && a.schedule.ModalOpen():
		return dialogSchedule
	default:
		return dialogNone
	}
}

func bump(signal *core.Signal[int]) { signal.Update(func(i int) int { return i + 1 }) }

// syncNative publishes only changed values. Showing the window or finishing a
// launch must preserve the mounted controls, their focus and scroll position.
func (a *App) syncNative() {
	n := a.native
	route := a.currentRoute()
	if a.library != nil {
		a.library.Language = route.language
	}
	a.schedule.Language = route.language
	n.route.Set(route)
	n.dialog.Set(a.currentDialog())
	n.message.Set(a.message)
	n.search.Set(a.searchOpen)
}

func (a *App) nativePage(cx *core.Scope) core.Node {
	n := a.native
	a.syncNative()
	page := w.Column(core.Dynamic(func(cx *core.Scope) core.Node {
		route := n.route.Get()
		if route.settings || a.launcherOnly() {
			return nil
		}
		return a.nativeNav(w.Page{Scope: cx, Refresh: a.syncNative})
	}), core.Dynamic(func(cx *core.Scope) core.Node {
		route := n.route.Get()
		v := w.Page{Scope: cx, Refresh: a.syncNative}
		if !route.settings && route.tab == 0 {
			return a.nativeLauncher(v)
		}
		n.content.Get()
		v.Refresh = func() {
			old := n.dialog.GetUntracked()
			a.syncNative()
			if n.route.GetUntracked() == route && old == n.dialog.GetUntracked() {
				bump(n.content)
			}
		}
		switch {
		case route.settings:
			return a.nativeSettings(v)
		case route.tab == 1 && a.library != nil:
			return a.library.Native(v)
		case route.tab == 2:
			return a.schedule.Native(v)
		default:
			return a.nativeLauncher(v)
		}
	}), core.Dynamic(func(cx *core.Scope) core.Node {
		if msg := n.message.Get(); msg != "" {
			v := w.Page{Scope: cx, Refresh: a.syncNative}
			return w.Row(w.Text(msg), v.Button(a.text("清除提示", "Dismiss"), func() { a.message = "" }))
		}
		return nil
	}), core.Dynamic(a.dialogLayer))
	page.Style.Padding, page.Style.Gap = core.All(14), 12
	page.StyleFunc = func() core.Style {
		s := n.route.Get()
		return core.Style{Background: cx.Palette().Background, FontFamily: fontName(s.font), FontSize: core.Points(s.fontSize)}
	}
	return page
}

func (a *App) dialogLayer(cx *core.Scope) core.Node {
	n := a.native
	kind := n.dialog.Get()
	if kind == dialogNone {
		return nil
	}
	body := core.Dynamic(func(cx *core.Scope) core.Node {
		n.dialogContent.Get()
		v := w.Page{Scope: cx, Refresh: func() {
			a.syncNative()
			if kind == n.dialog.GetUntracked() {
				bump(n.dialogContent)
			}
		}}
		switch kind {
		case dialogLibrary:
			return a.library.NativeDialog(v)
		case dialogSchedule:
			return a.schedule.NativeDialog(v)
		default:
			return a.nativeDialog(v)
		}
	})
	return base.Dialog{DefaultOpen: true, NoCloseOnEscape: true, NoCloseOnBackdropPress: true,
		BackdropStyle: core.Style{Background: core.RGBA(0, 0.28)},
		PopupStyleFunc: func() core.Style {
			size := cx.Env().Window()
			return core.Style{Width: core.Pt(min(620, size.Width-40)), MaxHeight: core.Pt(size.Height - 40),
				Padding: core.All(18), Background: cx.Palette().Surface, BorderColor: cx.Palette().Border, BorderWidth: 1, CornerRadius: 10}
		}, Children: []core.Node{body}}
}

func (a *App) nativeNav(v w.Page) core.Node {
	cx := v.Scope
	s := a.state.Settings
	var nav []core.Node
	labels := []string{a.text("应用启动", "Launcher"), "MCP / Skills", a.text("定时任务", "Scheduled tasks")}
	for i, label := range labels {
		if i == 1 && s.HideLibraryTab || i == 2 && s.HideScheduleTab {
			continue
		}
		variant := nsw.ButtonGhost
		if a.mainTab == i {
			variant = nsw.ButtonPrimary
		}
		nav = append(nav, nsw.Button{Text: label, Variant: variant, OnPress: v.Action(func() { a.mainTab = i; a.settingsOpen = false })})
	}
	tabs := w.Row(nav...)
	if cx.Env().Window().Width >= 680 {
		mark := core.Image{Source: core.ImageSource{Data: branding.PNG}, ResizeMode: core.Contain, Style: core.Style{Width: core.Pt(26), Height: core.Pt(26), Margin: core.Edges{Right: 6}}}
		tabs.Children = append([]core.Node{mark}, tabs.Children...)
	}
	header := w.Spread(tabs, v.Button(a.text("设置", "Settings"), a.openSettings))
	header.Style.Padding = core.Edges{Bottom: 10}
	header.Style.Borders = core.Edges{Bottom: 1}
	header.Style.BorderColor = cx.Palette().Border
	return header
}

func (a *App) nativeLauncher(v w.Page) core.Node {
	t := a.text
	cx := v.Scope
	q := core.NewSignal(cx, a.query)
	search := v.Action(func() {
		if a.currentDialog() == dialogNone {
			a.searchOpen = true
		}
	})
	cx.OnKey("mod+f", search)
	cx.OnKey("escape", v.Action(func() {
		if a.searchOpen && a.currentDialog() == dialogNone {
			a.searchOpen = false
			a.query = ""
			a.filter()
			q.Set("")
		}
	}))
	run := func() {
		if len(a.results) > 0 && a.currentDialog() == dialogNone {
			a.launch(a.results[a.selected])
		}
	}
	cx.OnKey("enter", run)
	cx.OnKey("space", run)
	var nodes []core.Node
	nodes = append(nodes, core.Show{When: a.native.search.Get, Then: func(*core.Scope) core.Node {
		return nsw.Input{TestID: "launcher-search", Label: t("搜索", "Search"), Placeholder: t("搜索所有分类…", "Search all groups…"), AutoFocus: true, DefaultValue: a.query, OnChange: func(s string) { a.query = s; a.filter(); q.Set(s) }, OnSubmit: func(string) { run() }}
	}})
	nodes = append(nodes, core.Dynamic(func(area *core.Scope) core.Node {
		q.Get()
		a.native.groups.Get()
		return a.nativeGroups(w.Page{Scope: area, Refresh: v.Refresh})
	}))
	return w.Column(nodes...)
}

type dropTarget struct {
	ref           *core.Ref
	group, before string
	index         int
}

func (a *App) nativeGroups(v w.Page) core.Node {
	cx := v.Scope
	t := a.text
	s := a.state.Settings
	targets := map[string]dropTarget{}
	preview := core.NewSignal(cx, "")
	gridRef, listRef := new(core.Ref), new(core.Ref)
	selected := core.NewSignal(cx, a.selected)
	var groups []core.Node
	for _, g := range a.state.Groups {
		disabled := strings.TrimSpace(a.query) != "" && a.matches[g.ID] == 0
		label := fmt.Sprintf("%s (%d)", g.Name, a.matches[g.ID])
		ref := new(core.Ref)
		first := ""
		if len(g.Apps) > 0 {
			first = g.Apps[0].ID
		}
		targets["group-"+g.ID] = dropTarget{ref: ref, group: g.ID, before: first, index: -1}
		group := core.View{Ref: ref, Label: label, Role: core.RoleButton, Disabled: func() bool { return disabled }, Style: core.Style{Padding: core.All(7)}, Children: []core.Node{w.Text(label)}, OnPress: func(core.PressEvent) { a.chooseGroup(g.ID) }, OnFileDrop: a.nativeDrop(g.ID, ""),
			OnContextMenu: func(e core.ContextMenuEvent) {
				cx.PopupMenu([]core.MenuItem{
					{Label: t("重命名分类", "Rename group"), OnSelect: v.Action(func() { a.renameID = g.ID; a.renameText = g.Name; a.renameOpen = true })},
					{Label: t("移除分类", "Remove group"), Disabled: len(a.state.Groups) == 1, OnSelect: func() { a.removeGroup(g.ID) }},
				}, e.PagePosition, nil)
			}}
		if g.ID == a.state.ActiveGroupID {
			group.Style.Background = cx.Palette().SurfacePressed
			group.Style.Color = cx.Palette().Accent
			group.Style.FontWeight = core.FontWeightSemibold
		}
		if disabled {
			group.Style.Color = cx.Palette().TextMuted
		}
		group.StyleFunc = func() core.Style {
			if preview.Get() == "group-"+g.ID {
				return core.Style{BorderWidth: 2, BorderColor: cx.Palette().Accent}
			}
			return core.Style{}
		}
		group.Style.CornerRadius = 6
		groups = append(groups, group)
	}
	sidebar := v.Panel(w.Heading(t("分类", "Groups")), core.Scroll{Style: core.Style{FlexGrow: 1}, Children: groups}, v.Button(t("新建分类", "New group"), a.addGroup))
	sidebar.Style.Padding = core.All(8)
	if a.launcherOnly() {
		sidebar.Children = append(sidebar.Children, v.Button(t("设置", "Settings"), a.openSettings))
	}
	sidebar.OnContextMenu = func(e core.ContextMenuEvent) {
		cx.PopupMenu([]core.MenuItem{{Label: t("添加分类", "Add group"), OnSelect: a.addGroup}}, e.PagePosition, nil)
	}
	sidebar.Style.FlexGrow = 0
	sidebar.Style.Width = core.Pt(core.Points(s.SidebarWidth))
	divider := core.View{Style: core.Style{Width: core.Pt(5), Background: cx.Palette().Background}, OnPan: func(e core.PanEvent) {
		if e.Phase == core.GestureEnded {
			a.state.Settings.SidebarWidth = s.SidebarWidth + float64(e.Translation.X)
			a.changed()
		}
	}}
	columns := max(1, int((float64(cx.Env().Window().Width)-s.SidebarWidth-77)/(s.CardWidth+6)))
	for key, step := range map[string]int{"left": -1, "right": 1, "up": -columns, "down": columns} {
		cx.OnKey(key, func() {
			if len(a.results) == 0 || a.currentDialog() != dialogNone {
				return
			}
			a.selected = max(0, min(len(a.results)-1, a.selected+step))
			selected.Set(a.selected)
			listRef.ScrollToIndexAt(a.selected, 0.5, 0, false)
		})
	}
	entries := append([]model.Entry(nil), a.results...)
	active := a.state.ActiveGroupID
	cards := w.List(len(entries), columns, core.Points(s.CardHeight), func(i int) any { return entries[i].ID }, func(i int) core.Node {
		e := entries[i]
		ref := new(core.Ref)
		targets[e.ID] = dropTarget{ref: ref, group: active, before: e.ID, index: i}
		cx.Cleanup(func() {
			if targets[e.ID].ref == ref {
				delete(targets, e.ID)
			}
		})
		card := core.View{Ref: ref, Role: core.RoleButton, Label: e.Name, Style: core.Style{Width: core.Pt(core.Points(s.CardWidth)), Height: core.Pt(core.Points(s.CardHeight)), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Background: cx.Palette().Surface, BorderWidth: 1, BorderColor: cx.Palette().Border, CornerRadius: 8},
			Children: []core.Node{a.nativeIcon(cx, e), core.Text{Content: e.Name, MaxLines: 1, Style: core.Style{FontSize: core.Points(s.CardFontSize)}}},
			OnPress:  func(core.PressEvent) { a.selected = i; selected.Set(i); a.launch(e) }, OnFileDrop: a.nativeDrop(active, e.ID),
			OnContextMenu: func(ev core.ContextMenuEvent) {
				cx.PopupMenu([]core.MenuItem{
					{Label: t("打开", "Open"), OnSelect: func() { a.launch(e) }}, {Label: t("打开所在文件夹", "Open containing folder"), OnSelect: func() { go func() { err := platform.OpenFolder(e.Path); a.post(func() { a.report(err) }) }() }},
					{Label: t("编辑", "Edit"), OnSelect: v.Action(func() { a.openEditor(e) })}, {Label: t("移除", "Remove"), OnSelect: func() { a.removeEntry(e.ID) }},
				}, ev.PagePosition, nil)
			}}
		hover := core.NewSignal(cx, false)
		card.OnMouseEnter = func(core.MouseEvent) { hover.Set(true) }
		card.OnMouseLeave = func(core.MouseEvent) { hover.Set(false) }
		card.StyleFunc = func() core.Style {
			style := core.Style{}
			if hover.Get() {
				style.Background = cx.Palette().SurfaceHover
			}
			if selected.Get() == i {
				style.BorderColor = cx.Palette().Focus
			}
			if preview.Get() == e.ID {
				style.BorderWidth = 2
				style.BorderColor = cx.Palette().Accent
			}
			return style
		}
		if strings.TrimSpace(a.query) == "" {
			var start core.Point
			card.OnPressDown = func(ev core.PressEvent) { start = ev.PagePosition }
			card.OnPan = func(ev core.PanEvent) {
				if ev.Phase == core.GestureCancelled {
					preview.Set("")
					return
				}
				at := core.Point{X: start.X + ev.Translation.X, Y: start.Y + ev.Translation.Y}
				dst, cue, ok := cardDestination(at, gridRef, targets, entries, active)
				if ev.Phase == core.GestureEnded {
					preview.Set("")
					if ok {
						a.moveEntry(e.ID, dst.group, dst.before)
					}
				} else {
					preview.Set(cue)
				}
			}
		}
		return card
	})
	cards.Ref = listRef
	cards.Empty = w.Text(t("拖入文件，或使用上方按钮添加快捷方式。", "Drop files here or add a shortcut using the buttons above."))
	if strings.TrimSpace(a.query) != "" {
		cards.Empty = w.Text(t("没有找到匹配的条目", "No matching shortcuts"))
	}
	groupName := ""
	if g := a.state.Group(active); g != nil {
		groupName = g.Name
	}
	header := w.Spread(w.Heading(groupName), v.Button(t("搜索", "Search"), func() { a.searchOpen = true }))
	tools := w.Row(v.Primary(t("添加文件", "Add files"), func() { a.nativePick(v, false) }), v.Button(t("添加目录", "Add folder"), func() { a.nativePick(v, true) }), v.Button(t("添加命令", "Add command"), a.newCommand), v.Button(a.installedAppsTitle(), a.openAddApps))
	grid := w.Column(cards)
	if len(entries) == 0 {
		grid.Children = []core.Node{cards.Empty}
		grid.Style.AlignItems = core.AlignCenter
		grid.Style.JustifyContent = core.JustifyCenter
		grid.Style.Color = cx.Palette().TextMuted
	}
	grid.Style.Borders = core.Edges{Top: 1}
	grid.Style.BorderColor = cx.Palette().Border
	grid.Style.Padding = core.Edges{Top: 12}
	grid.Ref = gridRef
	grid.OnContextMenu = func(ev core.ContextMenuEvent) {
		cx.PopupMenu([]core.MenuItem{
			{Label: t("添加文件", "Add files"), OnSelect: func() { a.nativePick(v, false) }},
			{Label: t("添加目录", "Add folder"), OnSelect: func() { a.nativePick(v, true) }},
			{Label: t("添加命令", "Add command"), OnSelect: v.Action(a.newCommand)},
			{Label: a.installedAppsTitle(), OnSelect: v.Action(a.openAddApps)},
		}, ev.PagePosition, nil)
	}
	grid.StyleFunc = func() core.Style {
		if preview.Get() == "$end" {
			return core.Style{BorderWidth: 1, BorderColor: cx.Palette().Accent}
		}
		return core.Style{}
	}
	grid.OnFileDrop = a.nativeDrop(active, "$end")
	grid.OnPress = func(ev core.PressEvent) {
		if ev.Clicks == 2 && s.DblClickBlankToHide {
			a.hide()
		}
	}
	content := v.Panel(header, tools, grid)
	content.Style.FlexGrow = 1
	content.Style.FlexBasis = core.Pt(0)
	content.Style.FlexShrink = 1
	return core.View{Style: core.Style{Direction: core.Row, FlexGrow: 1, Gap: 3}, Children: []core.Node{sidebar, divider, content}}
}

func (a *App) nativeIcon(cx *core.Scope, e model.Entry) core.Node {
	size := core.Points(a.state.Settings.CardIconScale * 0.72)
	initial, _ := utf8.DecodeRuneInString(e.Name)
	fallback := core.Text{Content: strings.ToUpper(string(initial)), Style: core.Style{FontSize: size * 0.6, Color: cx.Palette().Accent}}
	if strings.HasPrefix(strings.ToLower(e.Path), "command:") {
		return fallback
	}
	path := e.Icon
	if path == "" {
		path = e.Path
	}
	cached := a.native.icons[path]
	if cached == nil {
		surface := a.native.surface
		cached = core.NewSignalFunc[[]byte](surface, nil, nil)
		a.native.icons[path] = cached
		go func() {
			a.iconSlots <- struct{}{}
			data, err := platform.Icon(path, 128)
			<-a.iconSlots
			if err != nil {
				log.Printf("icon %s: %v", path, err)
				return
			}
			surface.Post(func() { cached.Set(data) })
		}()
	}
	return core.Dynamic(func(*core.Scope) core.Node {
		data := cached.Get()
		if len(data) == 0 {
			return fallback
		}
		return core.Image{Source: core.ImageSource{Data: data}, ResizeMode: core.Contain, Style: core.Style{Width: core.Pt(size), Height: core.Pt(size)}}
	})
}

func (a *App) nativeDrop(group, before string) func([]string) {
	return func(raw []string) {
		var paths []string
		for _, s := range raw {
			path, err := droppedPath(s)
			if err != nil {
				a.report(err)
				continue
			}
			paths = append(paths, path)
		}
		a.importPaths(paths, group, before)
	}
}

// droppedPath accepts desktop file URLs for both files and directories.
func droppedPath(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("invalid file URL %q", raw)
	}
	path := u.Path
	if runtime.GOOS == "windows" {
		if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
			path = "//" + u.Host + path
		} else {
			path = strings.TrimPrefix(path, "/")
		}
	} else if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
		return "", fmt.Errorf("unsupported file host %q", u.Host)
	}
	path = filepath.FromSlash(path)
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("expected absolute file path in %q", raw)
	}
	return path, nil
}

func (a *App) launcherOnly() bool {
	return a.state.Settings.HideLibraryTab && a.state.Settings.HideScheduleTab
}

func fontName(name string) string {
	switch name {
	case "maye":
		if runtime.GOOS == "windows" {
			return "Segoe UI"
		}
		return ""
	case "segoe":
		return "Segoe UI"
	case "system":
		return ""
	case "inter":
		return "Inter"
	case "mono":
		switch runtime.GOOS {
		case "windows":
			return "Consolas"
		case "darwin":
			return "Menlo"
		default:
			return "monospace"
		}
	}
	return name
}
