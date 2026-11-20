package launcher

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"my-quickstart-go/internal/platform"
)

func TestCardHighlightFollowsHoverAndKeyboardFocus(t *testing.T) {
	for _, theme := range themes {
		for _, size := range []struct {
			width int
			scale float32
		}{{560, 1}, {560, 2}, {960, 1}, {960, 2}} {
			scale := size.scale
			t.Run(fmt.Sprintf("%s/%d/%.0fx", theme.id, size.width, scale), func(t *testing.T) {
				a := fixture()
				a.state.Settings.Theme = theme.id
				a.busy = true
				tt := ui.NewTester(a.view, size.width, 620)
				tt.SetScale(scale)
				other, ok := tt.Find("Another App")
				if !ok {
					t.Fatal("reference card missing")
				}
				idleBorder := tt.Image().RGBAAt(int((other.X+2.5)*scale), int((other.Y+other.H/2)*scale))
				check := func(name string, hovered, focused bool) {
					t.Helper()
					r, ok := tt.Find(name)
					if !ok {
						t.Fatal("card missing: " + name)
					}
					y := int((r.Y + r.H/2) * scale)
					img := tt.Image()
					outside := img.RGBAAt(int((r.X-2)*scale), y)
					inside := img.RGBAAt(int((r.X+0.5)*scale), y)
					if inside != outside {
						t.Fatalf("extra outer selection at %q: %v, background %v", name, inside, outside)
					}
					want := palette(theme.id).Surface
					if hovered {
						want = palette(theme.id).SurfaceHover
					}
					body := img.RGBAAt(int((r.X+10)*scale), y)
					if body.R != want.R || body.G != want.G || body.B != want.B {
						t.Fatalf("card theme changed: %v, want %v", body, want)
					}
					border := img.RGBAAt(int((r.X+2.5)*scale), y)
					if (border != idleBorder) != (hovered || focused) {
						t.Fatalf("card %q highlight must follow hover or keyboard focus: border %v, idle %v, hovered %v, focused %v", name, border, idleBorder, hovered, focused)
					}
				}
				check("Alpha App", false, false)
				r, _ := tt.Find("Alpha App")
				tt.Move(r.X+10, r.Y+10)
				check("Alpha App", true, false)
				tt.Move(other.X+10, other.Y+10)
				check("Alpha App", false, false)
				check("Another App", true, false)
				tt.Move(float32(size.width-20), 580)
				check("Alpha App", false, false)
				tt.ClickAt(r.X+0.5, r.Y+r.H/2)
				check("Alpha App", false, false)
				tt.Key(0, ui.KeyRight)
				if a.selected != 1 {
					t.Fatal("grid arrow navigation must remain available")
				}
				check("Alpha App", false, false)
				check("Another App", false, false)
				tt.Key(0, ui.KeyTab)
				tt.Key(ui.Shift, ui.KeyTab)
				check("Another App", false, true)
				tt.Key(0, ui.KeyLeft)
				if a.selected != 0 {
					t.Fatal("keyboard focus must keep grid arrow navigation available")
				}
				check("Alpha App", false, true)
				check("Another App", false, false)
				tt.Key(0, ui.KeyRight)
				check("Another App", false, true)
				tt.SetFocused(false)
				check("Another App", false, false)
				tt.SetFocused(true)
				check("Another App", false, true)
				// Grid cells and their nested buttons share the accessible name.
				// Click the visible card, avoiding the wrapper's label lookup.
				tt.ClickAt(other.X+other.W/2, other.Y+other.H/2)
				tt.Move(float32(size.width-20), 580)
				check("Another App", false, false)
			})
		}
	}
}

func (a *App) sidebarView(c *ui.Context) {
	side := ui.Column(c).Fill().Padding(10).Gap(8).Background(c.Theme().Surface.Alpha(0.5))
	side.ContextMenu(func(m *ui.Menu) {
		if m.Item(a.text("添加分类", "Add group")).Chosen() {
			a.deferAction(a.addGroup)
		}
	})
	side.Children(func() {
		a.groups.Key = func(i int) any { return a.state.Groups[i].ID }
		ui.List(c, &a.groups, len(a.state.Groups), func(i int) {
			g := a.state.Groups[i]
			row := ui.ButtonBase(c.Key(g.ID)).Label(g.Name).Tooltip(g.Name).WidthPercent(100).Height(40).Padding(10).Radius(8)
			noMatches := strings.TrimSpace(a.query) != "" && a.matches[g.ID] == 0
			row.Disabled(noMatches)
			active := g.ID == a.state.ActiveGroupID && !noMatches
			accent := c.Theme().Accent
			if noMatches {
				gray := uint8(96)
				if c.Theme().Dark {
					gray = 144
				}
				accent = ui.RGB(gray, gray, gray)
			}
			if active {
				row.Background(accent.Alpha(0.14)).Border(1, accent.Alpha(0.25))
			} else if row.Hovered() {
				row.Background(c.Theme().SurfaceHover)
			}
			row.Children(func() {
				ui.Row(c).Fill().Gap(6).Children(func() {
					color := c.Theme().Text
					countColor := c.Theme().TextMuted
					if noMatches {
						color, countColor = accent, accent
					}
					ui.Text(c, g.Name).SingleLine().TextColor(color).Grow(1)
					ui.Text(c, fmt.Sprint(a.matches[g.ID])).FontSize(10).TextColor(countColor)
				})
			})
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
		if a.launcherOnly() {
			ui.Divider(c)
			if ui.Button(c, a.text("设置", "Settings")).FillWidth().Height(36).Disabled(a.editorOpen || a.renameOpen || a.addOpen).Clicked() {
				a.openSettings()
			}
		}
	})
}

func (a *App) gridView(c *ui.Context) {
	a.grid.Key = func(i int) any { return a.results[i].ID }
	a.grid.Label = func(i int) string { return a.results[i].Name }
	a.grid.Selected = &a.selected
	if len(a.results) == 0 {
		ui.Column(c).AlignItems(ui.Center).Gap(8).Padding(30, 16).Children(func() {
			title, hint := a.text("从一个快捷方式开始", "Your shortcuts belong here"), a.text("右键空白处添加，或将文件拖入下方区域。", "Right-click the empty space to add, or drop files below.")
			if strings.TrimSpace(a.query) != "" {
				title, hint = a.text("没有找到匹配的条目", "No matching shortcuts"), a.text("试试其他名称，或清空搜索查看分类。", "Try another name or clear the search to browse groups.")
			}
			ui.Text(c, title).FontSize(17).Bold()
			ui.Text(c, hint).TextColor(c.Theme().TextMuted)
		})
	}
	var slots []dragSlot
	// shortcut: GridView has no cell style hook; use a clear theme until MyGo exposes one.
	theme := c.Theme()
	gridTheme := *theme
	gridTheme.Accent, gridTheme.Border = ui.Color{}, ui.Color{}
	gridTheme.SurfacePressed, gridTheme.SurfaceHover = ui.Color{}, ui.Color{}
	c.SetTheme(&gridTheme)
	grid := ui.GridView(c, &a.grid, len(a.results), float32(a.state.Settings.CardWidth), float32(a.state.Settings.CardHeight), func(i int) {
		c.SetTheme(theme)
		slots = append(slots, a.card(c, i))
		c.SetTheme(&gridTheme)
	}).Grow(1).Padding(12, 20, 20, 20).Gap(10)
	c.SetTheme(theme)
	grid.ContextMenu(a.addMenu)
	if grid.Submitted() && a.selected < len(a.results) {
		e := a.results[a.selected]
		a.deferAction(func() { a.launch(e) })
	}
	if grid.DoubleClicked() && a.state.Settings.DblClickBlankToHide {
		a.deferAction(a.hide)
	}
	if strings.TrimSpace(a.query) == "" {
		a.previewReorder(c, grid, slots)
	}
	if paths := grid.DroppedFiles(); len(paths) > 0 {
		a.importPaths(paths, a.state.ActiveGroupID, "$end")
	}
}

func (a *App) card(c *ui.Context, i int) dragSlot {
	e := a.results[i]
	slot := ui.Box(c).FillWidth().Height(float32(a.state.Settings.CardHeight))
	var card ui.Element
	slot.Children(func() {
		card = ui.ButtonBase(c.Key(e.ID)).Label(e.Name).Tooltip(e.Name).Size(float32(a.state.Settings.CardWidth), float32(a.state.Settings.CardHeight)).Radius(10).Padding(6).Background(c.Theme().Surface).Border(1, c.Theme().Border.Alpha(0.6))
		if i == a.selected && c.Resolve(a.grid.Handle).FocusVisible() {
			card.Border(1, c.Theme().Accent.Alpha(0.65))
		}
		if strings.TrimSpace(a.query) == "" {
			card.Drag(entryDrag{e.ID})
		}
		if card.Hovered() {
			card.Background(c.Theme().SurfaceHover).Border(1, c.Theme().Accent)
		}
		card.Children(func() {
			ui.Column(c).Fill().Center().Gap(4).Children(func() {
				size := float32(a.state.Settings.CardIconScale * 0.72)
				if icon := a.icon(e); icon != nil {
					ui.Image(c, icon).Size(size, size)
				} else {
					r, _ := utf8.DecodeRuneInString(e.Name)
					ui.Box(c).Size(size, size).Radius(size * 0.25).Background(c.Theme().Accent.Alpha(0.12)).Center().Children(func() {
						ui.Text(c, strings.ToUpper(string(r))).FontSize(size * 0.55).Bold().TextColor(c.Theme().Accent)
					})
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
	})
	if card.FileDragOver() {
		card.Border(2, c.Theme().Accent)
	}
	if paths := card.DroppedFiles(); len(paths) > 0 {
		a.importPaths(paths, a.state.ActiveGroupID, e.ID)
	}
	return dragSlot{index: i, bounds: slot.Bounds(), card: card}
}

func (a *App) addMenu(m *ui.Menu) {
	if m.Item(a.text("添加应用 / 文件", "Add application / file")).Chosen() {
		a.deferAction(func() { a.pickFiles(false) })
	}
	if m.Item(a.text("添加命令", "Add command")).Chosen() {
		a.newCommand()
	}
	if m.Item(a.installedAppsTitle()).Chosen() {
		a.openAddApps()
	}
}
