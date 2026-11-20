package launcher

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"my-quickstart-go/internal/model"
)

func TestSearchSelectsFirstMatchingTabAndFiltersThatGroup(t *testing.T) {
	a := fixture()
	a.state.Groups[1].Apps = append(a.state.Groups[1].Apps, model.NewEntry("Beta App", "beta-app.exe", ""))
	a.filter()
	original := a.state.ActiveGroupID
	first, second, empty := a.state.Groups[0].ID, a.state.Groups[1].ID, a.state.Groups[2].ID
	a.chooseGroup(second)
	tt := ui.NewTester(a.view, 960, 620)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("app")
	if a.state.ActiveGroupID != first || len(a.results) != 2 || tt.HasText("Beta App") {
		t.Fatalf("new queries must select the first matching tab and show its entries: %q", tt.Texts())
	}
	if err := tt.Click(a.state.Groups[1].Name); err != nil {
		t.Fatal(err)
	}
	if a.state.ActiveGroupID != second || len(a.results) != 1 || a.results[0].Name != "Beta App" || tt.HasText("Alpha App") {
		t.Fatal("choosing a matching tab must display only its matches")
	}
	a.chooseGroup(empty)
	tt.Frame()
	if a.state.ActiveGroupID != second {
		t.Fatal("empty result tabs must reject selection")
	}
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("alpha")
	if a.state.ActiveGroupID != first || a.matches[second] != 0 {
		t.Fatal("query changes must select the first matching group")
	}
	if err := tt.Click(a.state.Groups[1].Name); err != nil {
		t.Fatal(err)
	}
	if a.state.ActiveGroupID != first {
		t.Fatal("disabled zero-match tabs must not respond to clicks")
	}
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("no match")
	if len(a.results) != 0 {
		t.Fatal("an unmatched query must display an empty grid")
	}
	for _, g := range a.state.Groups {
		if err := tt.Click(g.Name); err != nil {
			t.Fatal(err)
		}
		if a.state.ActiveGroupID != first {
			t.Fatal("all zero-match tabs must stay disabled")
		}
	}
	tt.Key(0, ui.KeyEscape)
	if a.state.ActiveGroupID != second || len(a.results) != 2 {
		t.Fatal("closing search must restore the group selected before searching")
	}
	a.chooseGroup(empty)
	if a.state.ActiveGroupID != empty || len(a.results) != 0 {
		t.Fatal("empty groups become selectable after search closes")
	}
	a.chooseGroup(original)
	if a.state.ActiveGroupID != original {
		t.Fatal("normal group selection must still work")
	}
}

func (a *App) view(c *ui.Context) {
	t := palette(a.state.Settings.Theme)
	t.FontSize = float32(a.state.Settings.FontSize)
	t.Font = fontName(a.state.Settings.FontFamily)
	c.SetTheme(t)
	c.Root().Background(t.Background)
	if a.mainTab == 1 && a.state.Settings.HideLibraryTab || a.mainTab == 2 && a.state.Settings.HideScheduleTab {
		a.mainTab = 0
	}
	if a.mainTab == 0 && !a.settingsOpen && !a.editorOpen && !a.renameOpen && !a.addOpen {
		c.OnShortcut(ui.Cmd, ui.KeyF, func() {
			if a.searchOpen {
				c.Resolve(a.search).SetTextSelection(0, utf8.RuneCountInString(a.query))
			}
			a.searchOpen = true
			a.search.Focus(c)
		})
		c.OnShortcut(0, ui.KeyEscape, func() {
			a.query = ""
			a.searchOpen = false
			a.filter()
		})
	}
	ui.Column(c).Fill().Children(func() {
		a.mainTabsView(c)
		if a.library != nil {
			a.library.Language = a.state.Settings.Language
		}
		if a.settingsOpen {
			a.settingsView(c)
		} else if a.mainTab == 2 {
			a.schedule.Language = a.state.Settings.Language
			ui.Column(c).Grow(1).FillWidth().Children(func() {
				a.schedule.View(c)
				a.statusView(c)
			})
		} else if a.mainTab == 1 {
			if a.library != nil {
				a.library.Language = a.state.Settings.Language
				a.library.View(c)
			} else {
				ui.Text(c, a.text("资源库尚未初始化", "Library is not initialized"))
			}
		} else {
			ui.Column(c).Grow(1).FillWidth().Children(func() {
				if a.searchOpen {
					ui.Row(c).Padding(18, 20).Children(func() {
						input := ui.TextInput(c.Key("search"), &a.query).Bind(&a.search).Placeholder(a.text("搜索所有分类…", "Search all groups…")).Label(a.text("搜索", "Search")).Grow(1).Height(38)
						if input.Changed() {
							a.filter()
						}
						if input.Submitted() && a.selected >= 0 && a.selected < len(a.results) {
							e := a.results[a.selected]
							a.deferAction(func() { a.launch(e) })
						}
					})
					ui.Divider(c)
				}
				if ui.Split(c, &a.sidebar, func() { a.sidebarView(c) }, func() {
					ui.Column(c).Fill().Children(func() {
						if strings.TrimSpace(a.query) != "" {
							ui.Text(c, fmt.Sprintf(a.text("搜索结果（%d）", "Search results (%d)"), len(a.results))).FontSize(11).TextColor(t.TextMuted).Padding(12, 20, 0, 20)
						}
						a.gridView(c)
					})
				}).Grow(1).Changed() {
					a.state.Settings.SidebarWidth = float64(a.sidebar)
					a.changed()
				}
				a.statusView(c)
			})
		}
	})

	a.editorView(c)
	a.renameView(c)
	a.addAppsView(c)
	actions := a.pending
	a.pending = nil
	for _, fn := range actions {
		fn()
	}
}

func (a *App) mainTabsView(c *ui.Context) {
	if a.launcherOnly() {
		return
	}
	labels := []string{a.text("应用启动", "Launcher")}
	ids := []int{0}
	if !a.state.Settings.HideLibraryTab {
		labels, ids = append(labels, "MCP / Skills"), append(ids, 1)
	}
	if !a.state.Settings.HideScheduleTab {
		labels, ids = append(labels, a.text("定时任务", "Scheduled tasks")), append(ids, 2)
	}
	selected := 0
	for i, id := range ids {
		if id == a.mainTab {
			selected = i
		}
	}
	ui.Row(c).Label(a.text("主导航", "Main navigation")).FillWidth().AlignItems(ui.Center).Children(func() {
		disabled := a.editorOpen || a.renameOpen || a.addOpen || (a.mainTab == 2 && a.schedule.ModalOpen())
		if a.settingsOpen {
			for i, label := range labels {
				button := ui.ButtonBase(c.Key("settings-return-"+label)).Padding(10, 12).Label(label).Disabled(disabled)
				button.Children(func() { ui.Text(c, label).TextColor(c.Theme().TextMuted) })
				if button.Clicked() {
					a.mainTab, a.settingsOpen = ids[i], false
				}
			}
		} else if ui.Tabs(c, &selected, labels...).Disabled(disabled).Changed() {
			a.mainTab, a.settingsOpen = ids[selected], false
		}
		ui.Box(c).Grow(1)
		button := ui.Button(c, a.text("设置", "Settings")).Margin(0, 12).Disabled(disabled)
		if a.settingsOpen {
			button.Background(c.Theme().Accent.Alpha(0.14))
		}
		if button.Clicked() {
			a.openSettings()
		}
	})
	ui.Divider(c)
}
