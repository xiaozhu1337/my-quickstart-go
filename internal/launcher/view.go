package launcher

import (
	"fmt"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/platform"
)

func (a *App) view(c *ui.Context) {
	t := ui.DarkTheme()
	if a.state.Settings.Theme == "light" {
		t = ui.LightTheme()
	}
	t.FontSize = float32(a.state.Settings.FontSize)
	t.Font = fontName(a.state.Settings.FontFamily)
	c.SetTheme(t)
	c.Root().Background(t.Background)
	c.OnShortcut(ui.Cmd, ui.KeyF, func() {
		a.search.Focus()
		c.Resolve(a.search).SetTextSelection(0, utf8.RuneCountInString(a.query))
	})
	c.OnShortcut(0, ui.KeyEscape, func() {
		if a.query != "" {
			a.query = ""
			a.filter()
		}
	})
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(12).Gap(8).AlignItems(ui.Center).Children(func() {
			input := ui.TextInput(c.Key("search"), &a.query).Bind(&a.search).Placeholder(a.text("搜索所有分类中的应用…", "Search all groups…")).Label(a.text("搜索", "Search")).Grow(1)
			if input.Changed() {
				a.filter()
			}
			if input.Submitted() && a.selected < len(a.results) {
				e := a.results[a.selected]
				a.deferAction(func() { a.launch(e) })
			}
			ui.Button(c, a.text("添加", "Add")).Menu(func(m *ui.Menu) { a.addMenu(m) })
		})
		ui.Divider(c)
		if ui.Split(c, &a.sidebar, func() { a.sidebarView(c) }, func() { a.gridView(c) }).Grow(1).Changed() {
			a.state.Settings.SidebarWidth = float64(a.sidebar)
			a.changed()
		}
		ui.Divider(c)
		ui.Row(c).Padding(6, 12).Gap(12).Children(func() {
			msg := a.message
			if msg == "" {
				msg = fmt.Sprintf(a.text("%d 个条目", "%d items"), len(a.results))
			}
			ui.Text(c, msg).FontSize(11).Grow(1).SingleLine().Tooltip(msg)
			if a.message != "" && ui.Button(c, a.text("清除提示", "Dismiss")).Clicked() {
				a.message = ""
			}
			if a.registeredHotkey != "" {
				ui.Text(c, a.registeredHotkey).FontSize(11).TextColor(t.TextMuted)
			}
		})
	})
	a.editorView(c)
	a.renameView(c)
	a.addAppsView(c)
	a.settingsView(c)
	actions := a.pending
	a.pending = nil
	for _, fn := range actions {
		fn()
	}
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

func (a *App) sidebarView(c *ui.Context) {
	side := ui.Column(c).Fill().Padding(8).Gap(8)
	side.ContextMenu(func(m *ui.Menu) {
		if m.Item(a.text("添加分类", "Add group")).Chosen() {
			a.deferAction(a.addGroup)
		}
	})
	side.Children(func() {
		a.groups.Key = func(i int) any { return a.state.Groups[i].ID }
		ui.List(c, &a.groups, len(a.state.Groups), func(i int) {
			g := a.state.Groups[i]
			row := ui.Button(c.Key(g.ID), g.Name).Label(g.Name).Tooltip(g.Name).WidthPercent(100)
			if g.ID == a.state.ActiveGroupID {
				row.Background(c.Theme().Accent.Alpha(0.2))
			}
			if row.Clicked() {
				a.deferAction(func() { a.chooseGroup(g.ID) })
			}
			row.ContextMenu(func(m *ui.Menu) {
				if m.Item(a.text("重命名", "Rename")).Chosen() {
					a.renameID = g.ID
					a.renameText = g.Name
					a.renameOpen = true
				}
				if m.Item(a.text("移除分类", "Remove group")).Disabled(len(a.state.Groups) <= 1).Chosen() {
					a.deferAction(func() { a.removeGroup(g.ID) })
				}
			})
			if over, ok := ui.DragOver[entryDrag](row); ok && over.ID != "" {
				row.Border(2, c.Theme().Accent)
			}
			if drag, ok := ui.Drop[entryDrag](row); ok {
				before := ""
				if len(g.Apps) > 0 {
					before = g.Apps[0].ID
				}
				a.deferAction(func() { a.moveEntry(drag.ID, g.ID, before) })
			}
			if row.FileDragOver() {
				row.Border(2, c.Theme().Accent)
			}
			if paths := row.DroppedFiles(); len(paths) > 0 {
				a.importPaths(paths, g.ID, "")
			}
		}).Grow(1)
		if ui.Button(c, a.text("新建分类", "New group")).WidthPercent(100).Clicked() {
			a.deferAction(a.addGroup)
		}
		if ui.Button(c, a.text("设置", "Settings")).WidthPercent(100).Clicked() {
			a.openSettings()
		}
	})
}

func (a *App) gridView(c *ui.Context) {
	a.grid.Key = func(i int) any { return a.results[i].ID }
	a.grid.Label = func(i int) string { return a.results[i].Name }
	a.grid.Selected = &a.selected
	if len(a.results) == 0 {
		ui.Text(c, a.text("右键或点击“添加”创建快捷启动条目，也可以拖入文件。", "Use Add or drop files here to create shortcuts.")).Padding(16).TextColor(c.Theme().TextMuted)
	}
	grid := ui.GridView(c, &a.grid, len(a.results), float32(a.state.Settings.CardWidth), float32(a.state.Settings.CardHeight), func(i int) { a.card(c, i) }).Grow(1).Padding(12).Gap(8)
	grid.ContextMenu(a.addMenu)
	if grid.Submitted() && a.selected < len(a.results) {
		e := a.results[a.selected]
		a.deferAction(func() { a.launch(e) })
	}
	if grid.DoubleClicked() && a.state.Settings.DblClickBlankToHide {
		a.deferAction(a.hide)
	}
	if _, over := ui.DragOver[entryDrag](grid); over {
		grid.Border(1, c.Theme().Accent)
	}
	if drag, ok := ui.Drop[entryDrag](grid); ok {
		groupID := a.state.ActiveGroupID
		a.deferAction(func() { a.moveEntry(drag.ID, groupID, "") })
	}
	if paths := grid.DroppedFiles(); len(paths) > 0 {
		a.importPaths(paths, a.state.ActiveGroupID, "$end")
	}
}

func (a *App) card(c *ui.Context, i int) {
	e := a.results[i]
	card := ui.ButtonBase(c.Key(e.ID)).Label(e.Name).Tooltip(e.Name).Size(float32(a.state.Settings.CardWidth), float32(a.state.Settings.CardHeight)).Radius(8).Padding(6).Background(c.Theme().Surface)
	if a.query == "" {
		card.Drag(entryDrag{e.ID})
	}
	if card.Hovered() {
		card.Background(c.Theme().SurfaceHover)
	}
	card.Children(func() {
		ui.Column(c).Fill().Center().Gap(4).Children(func() {
			size := float32(a.state.Settings.CardIconScale * 0.72)
			if icon := a.icon(e); icon != nil {
				ui.Image(c, icon).Size(size, size)
			} else {
				r, _ := utf8.DecodeRuneInString(e.Name)
				ui.Text(c, strings.ToUpper(string(r))).FontSize(size * 0.6).Height(size)
			}
			ui.Text(c, e.Name).FontSize(float32(a.state.Settings.CardFontSize)).SingleLine().MaxWidth(float32(a.state.Settings.CardWidth - 12))
		})
	})
	if card.Clicked() {
		a.selected = i
		a.deferAction(func() { a.launch(e) })
	}
	card.ContextMenu(func(m *ui.Menu) {
		if m.Item(a.text("打开", "Open")).Chosen() {
			a.deferAction(func() { a.launch(e) })
		}
		if m.Item(a.text("打开所在文件夹", "Open containing folder")).Chosen() {
			a.deferAction(func() { go func() { err := platform.OpenFolder(e.Path); a.window.Update(func() { a.report(err) }) }() })
		}
		if m.Item(a.text("编辑", "Edit")).Chosen() {
			a.openEditor(e)
		}
		if m.Item(a.text("移除", "Remove")).Chosen() {
			a.deferAction(func() { a.removeEntry(e.ID) })
		}
	})
	if a.query == "" {
		if _, ok := ui.DragOver[entryDrag](card); ok {
			card.Border(2, c.Theme().Accent)
		}
		if drag, ok := ui.Drop[entryDrag](card); ok {
			groupID := a.state.ActiveGroupID
			a.deferAction(func() { a.moveEntry(drag.ID, groupID, e.ID) })
		}
	}
	if card.FileDragOver() {
		card.Border(2, c.Theme().Accent)
	}
	if paths := card.DroppedFiles(); len(paths) > 0 {
		a.importPaths(paths, a.state.ActiveGroupID, e.ID)
	}
}

func (a *App) addMenu(m *ui.Menu) {
	if m.Item(a.text("添加应用 / 文件", "Add application / file")).Chosen() {
		a.deferAction(func() { a.pickFiles(false) })
	}
	if m.Item(a.text("添加命令", "Add command")).Chosen() {
		a.newCommand()
	}
	if m.Item(a.text("添加系统应用", "Add installed application")).Chosen() {
		a.openAddApps()
	}
}
