package launcher

import (
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"

	"my-quickstart-go/internal/model"
)

func TestNSGoPages(t *testing.T) {
	state := model.Default()
	state.Settings.Language = "en"
	state.Settings.ToggleHotkey = "ctrl+alt+b"
	a := New(state, nil)
	a.SetLibrary(t.TempDir())
	app := nsgotest.StartApp(t, a.Mount, nsgotest.Environment(core.Environment{OS: core.OSWindows, Idiom: core.IdiomDesktop}), nsgotest.Size(960, 620), nsgotest.HotKeyTaken("ctrl+alt+k"))
	defer a.Close()
	press := func(name string) { t.Helper(); app.Press("role:button " + name); app.Settle() }
	checkMount := func(selector string) func() {
		t.Helper()
		id := app.Find(selector).ID
		return func() {
			t.Helper()
			if app.Find(selector).ID != id {
				t.Fatalf("%s was remounted", selector)
			}
		}
	}
	checkNav := checkMount("role:button Settings")
	checkGroup := checkMount("role:button New group")
	press("Add command")
	checkNav()
	checkGroup()
	press("Cancel")
	checkGroup()
	press("Add command")
	app.Type("label:Name", "Example")
	app.Type("label:Command text", "echo example")
	press("Save")
	if len(a.results) != 1 || a.results[0].Name != "Example" {
		t.Fatalf("saved entries: %+v", a.results)
	}
	checkNav()
	press("Search")
	app.Type("label:Search", "Example")
	checkSearch := checkMount("id:launcher-search")
	a.post(func() { a.message = "Completed" })
	app.WaitFor("text:Completed", time.Second)
	checkSearch()
	press("Dismiss")
	checkSearch()
	app.Key("escape")
	press("MCP / Skills")
	checkLibrary := checkMount("role:button Add MCP")
	press("Add MCP")
	checkLibrary()
	app.Type("label:Name", "example-mcp")
	press("Save")
	app.WaitFor("label:example-mcp", 3*time.Second)
	checkResource := checkMount("label:example-mcp")
	app.Press("label:example-mcp")
	checkResource()
	app.Find("text:1 selected")
	press("Pin")
	app.WaitFor("role:button Unpin", 3*time.Second)
	app.Find("text:1 selected")
	press("Scheduled tasks")
	checkTasks := checkMount("role:button Add task")
	press("Add task")
	checkTasks()
	app.Type("label:Task name", "Example schedule")
	app.Type("label:Command (including arguments)", "echo scheduled")
	press("Save")
	if len(a.state.Tasks) != 1 || a.state.Tasks[0].Name != "Example schedule" || a.state.Tasks[0].Time != "09:00" {
		t.Fatalf("saved tasks: %+v", a.state.Tasks)
	}
	press("Settings")
	for _, name := range []string{"Layout", "Behavior", "Hotkey", "About", "Resource import", "Appearance"} {
		press(name)
		if name == "Hotkey" {
			app.Clear("label:Global shortcut")
			app.Type("label:Global shortcut", "ctrl+alt+k")
			press("Apply shortcut")
			if a.registeredHotkey != "ctrl+alt+b" || a.state.Settings.ToggleHotkey != a.registeredHotkey || !strings.Contains(a.message, "Shortcut unavailable") {
				t.Fatalf("rejected shortcut: registered=%q saved=%q message=%q", a.registeredHotkey, a.state.Settings.ToggleHotkey, a.message)
			}
		}
	}
	press("Back")
	if a.settingsOpen {
		t.Fatal("settings still open")
	}
}

func fixture() *App {
	s := model.Default()
	s.Settings.Language = "en"
	s.Groups[0].Apps = []model.Entry{model.NewEntry("Alpha App", "alpha.exe", "--one"), model.NewEntry("Another App", "another.exe", "")}
	s.Groups[1].Apps = []model.Entry{model.NewEntry("Beta Tool", "beta.exe", "")}
	return New(s, nil)
}

func TestEntryOperations(t *testing.T) {
	a := fixture()
	first := a.state.Groups[0].ID
	second := a.state.Groups[1].ID
	a.query = " BETA "
	a.filter()
	if len(a.results) != 1 || a.results[0].Name != "Beta Tool" {
		t.Fatal("search must include other groups and trim query")
	}
	e := a.results[0]
	a.openEditor(e)
	a.draft.Name = " Renamed "
	a.envText = "NO_EQUALS\nKEY=a=b\n   \nKEY="
	a.saveEditor()
	g, edited := a.state.Entry(e.ID)
	if g.ID != second || edited.Name != "Renamed" || !reflect.DeepEqual(edited.EnvVars, []string{"NO_EQUALS", "KEY=a=b", "KEY="}) {
		t.Fatalf("editing search result failed: %+v", edited)
	}
	a.query = ""
	a.moveEntry(e.ID, first, a.state.Groups[0].Apps[0].ID)
	if a.state.Groups[0].Apps[0].ID != e.ID || len(a.state.Groups[1].Apps) != 0 {
		t.Fatal("cross-group move failed")
	}
	a.moveEntry(e.ID, first, "")
	if a.state.Groups[0].Apps[2].ID != e.ID {
		t.Fatal("same-group move to end failed")
	}
	if n := a.addEntries(first, "", []model.Entry{a.state.Groups[0].Apps[0], a.state.Groups[0].Apps[0]}); n != 0 {
		t.Fatal("file duplicates must be rejected")
	}
	a.chooseGroup(second)
	a.removeGroup(second)
	if a.state.ActiveGroupID != first {
		t.Fatal("removing active group must choose the first group")
	}
	for len(a.state.Groups) > 1 {
		a.removeGroup(a.state.Groups[1].ID)
	}
	a.removeGroup(first)
	if len(a.state.Groups) != 1 {
		t.Fatal("last group must remain")
	}
}

func TestCommandEditor(t *testing.T) {
	a := fixture()
	a.newCommand()
	a.draft.Path = "  " + strings.Repeat("测", 41) + "  "
	a.saveEditor()
	e := a.state.Groups[0].Apps[0]
	if e.Name != strings.Repeat("测", 40)+"..." || e.Path != "command:"+strings.Repeat("测", 41) {
		t.Fatalf("command default name or trim failed: %+v", e)
	}
	original := e
	a.openEditor(e)
	a.draft.Path = "echo changed"
	a.draft.Args = "ignored"
	a.envText = "KEY=ignored"
	a.saveEditor()
	e = a.state.Groups[0].Apps[0]
	if e.ID != original.ID || e.AddedAt != original.AddedAt || e.Args != "" || len(e.EnvVars) != 0 {
		t.Fatal("command editing must preserve identity and clear app arguments")
	}
}

func TestNativeViewSearchGroupsAndEditor(t *testing.T) {
	a := fixture()
	tt := ui.NewTester(a.view, 960, 620)
	if !tt.HasText("Alpha App") {
		t.Fatalf("initial grid is missing: %q", tt.Texts())
	}
	if _, visible := tt.Find("Search"); visible || tt.HasText("Add") {
		t.Fatal("idle view must hide the search and top add button")
	}
	tt.Key(ui.Cmd, ui.KeyF)
	if !tt.Focused("Search") {
		t.Fatal("the shortcut must show and focus search")
	}
	tt.Type("beta")
	if len(a.results) != 1 || a.results[0].Name != "Beta Tool" {
		t.Fatalf("search input did not update results: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	if a.query != "" || a.searchOpen {
		t.Fatal("Escape must clear and hide search")
	}
	if _, visible := tt.Find("Search"); visible {
		t.Fatal("the search field must be removed after Escape")
	}
	if tt.HasText("New group") {
		t.Fatal("sidebar must not show the new group button")
	}
	tt.RightClickAt(20, 200)
	if err := tt.ChooseMenuItem("Add group"); err != nil {
		t.Fatal(err)
	}
	if len(a.state.Groups) != 4 {
		t.Fatal("sidebar context menu did not create a group")
	}
	if err := tt.RightClick("Group-1"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Rename"); err != nil {
		t.Fatal(err)
	}
	if !a.renameOpen {
		t.Fatal("rename menu did not open the dialog")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Renamed group")
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	if a.state.Groups[0].Name != "Renamed group" {
		t.Fatal("rename dialog did not save")
	}
	a.openEditor(a.state.Groups[0].Apps[0])
	tt.Frame()
	if err := tt.Click("Name"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Edited app")
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	if a.state.Groups[0].Apps[0].Name != "Edited app" {
		t.Fatal("entry dialog did not save")
	}
	if err := tt.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Behavior"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Hide window when the application starts"); err != nil {
		t.Fatal(err)
	}
	if !a.state.Settings.HideOnStartup {
		t.Fatal("behavior checkbox did not save")
	}
}

func TestContextualSearchAndMessages(t *testing.T) {
	for _, width := range []int{560, 960} {
		a := fixture()
		tt := ui.NewTester(a.view, width, 380)
		if tt.HasText("Dismiss") {
			t.Fatal("idle view must not reserve a message bar")
		}
		tt.Key(ui.Cmd, ui.KeyF)
		tt.Type("beta")
		if !tt.HasText("Search results (1)") || !tt.HasText("Beta Tool") {
			t.Fatalf("global search context is missing: %q", tt.Texts())
		}
		a.message = "Hotkey unavailable"
		tt.Frame()
		if !tt.HasText(a.message) {
			t.Fatal("errors must remain visible")
		}
		if err := tt.Click("Settings"); err != nil {
			t.Fatal(err)
		}
		if !tt.HasText(a.message) {
			t.Fatal("settings must preserve the error message")
		}
		if err := tt.Click("Dismiss"); err != nil {
			t.Fatal(err)
		}
		if a.message != "" || tt.HasText("Dismiss") {
			t.Fatal("dismissing a message must remove the bar")
		}
		if err := tt.Click("Close"); err != nil {
			t.Fatal(err)
		}
		if !tt.HasText("Beta Tool") || a.query != "beta" {
			t.Fatal("returning to the launcher must preserve search")
		}
	}
}

func TestMainTabsPreserveLauncherState(t *testing.T) {
	a := fixture()
	a.SetLibrary(t.TempDir())
	tt := ui.NewTester(a.view, 960, 620)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("beta")
	if err := tt.Click("MCP / Skills"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Add MCP") || tt.HasText("Beta Tool") || tt.HasText("Library") {
		t.Fatal("library tab did not replace launcher view")
	}
	tt.Key(0, ui.KeyEscape)
	if a.query != "beta" {
		t.Fatal("launcher shortcuts leaked into library")
	}
	if err := tt.Click("Launcher"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Beta Tool") || a.query != "beta" {
		t.Fatal("switching tabs lost launcher state")
	}
}

func TestGlobalSettingsAndMagpieImport(t *testing.T) {
	for _, width := range []int{560, 1100} {
		root := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", root)
		if err := os.Mkdir(filepath.Join(root, "magpie"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "magpie", "library.json"), []byte(`{"mcp":[{"name":"imported-tools","command":"test-tool"}],"skills":[]}`), 0600); err != nil {
			t.Fatal(err)
		}
		a := fixture()
		a.state.Settings.Language = "zh-CN"
		a.SetLibrary(filepath.Join(root, "resources"))
		tt := ui.NewTester(a.view, width, 680)
		r, ok := tt.Find("设置")
		if !ok || r.X < float32(width-130) || r.Y > 50 {
			t.Fatal("settings is not at the top right", r)
		}
		if err := tt.Click("MCP / Skills"); err != nil {
			t.Fatal(err)
		}
		if err := tt.Click("设置"); err != nil {
			t.Fatal(err)
		}
		chooseSettings(t, tt, a, width, 680, "资源导入")
		if err := tt.Click("从 magpie 导入 MCP 和 Skill"); err != nil {
			t.Fatal(err)
		}
		if !tt.HasText("已导入 1 个 MCP、0 个 Skill；跳过 0 项，失败 0 项。") {
			t.Fatal(tt.Texts())
		}
		if destination := os.Getenv("QUICKSTART_SETTINGS_PREVIEW"); destination != "" && width == 1100 {
			f, err := os.Create(destination)
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := tt.Click("关闭"); err != nil {
			t.Fatal(err)
		}
		if a.settingsOpen || a.mainTab != 1 || !tt.HasText("imported-tools") {
			t.Fatal("settings return lost tab or import")
		}
		if tt.HasText("资源库") {
			t.Fatal("large title is still visible")
		}
		for _, label := range []string{"MCP / Skills", "应用启动"} {
			if err := tt.Click("设置"); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			if a.settingsOpen {
				t.Fatal("tab click did not leave settings")
			}
		}
	}
}

func (a *App) editorView(c *ui.Context) {
	_, height := c.Size()
	title := a.text("编辑应用", "Edit application")
	if a.command {
		title = a.text("命令", "Command")
	}
	ui.Modal(c, &a.editorOpen, func() {
		ui.Text(c, title).FontSize(20).Bold()
		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Gap(10).Children(func() {
				ui.Text(c, a.text("名称", "Name"))
				ui.TextInput(c.Key("entry-name"), &a.draft.Name).Label(a.text("名称", "Name")).AutoFocus()
				if a.command {
					ui.Text(c, a.text("命令内容", "Command text"))
					ui.TextArea(c.Key("entry-command"), &a.draft.Path).Label(a.text("命令内容", "Command text")).Height(130)
				} else {
					ui.Text(c, a.text("路径", "Path"))
					ui.TextInput(c.Key("entry-path"), &a.draft.Path).Label(a.text("路径", "Path"))
					ui.Text(c, a.text("参数", "Arguments"))
					ui.TextInput(c.Key("entry-args"), &a.draft.Args).Label(a.text("参数", "Arguments"))
					ui.Text(c, a.text("环境变量（每行 KEY=value）", "Environment (one KEY=value per line)"))
					ui.TextArea(c.Key("entry-env"), &a.envText).Label(a.text("环境变量", "Environment")).Height(90)
				}
			})
		})
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, a.text("取消", "Cancel")).Clicked() {
				a.editorOpen = false
			}
			if ui.PrimaryButton(c, a.text("保存", "Save")).Disabled(a.command && strings.TrimSpace(a.draft.Path) == "").Clicked() {
				a.deferAction(a.saveEditor)
			}
		})
	}).Label(title).Width(720).Height(min(540, height-40))
}

func (a *App) renameView(c *ui.Context) {
	ui.Modal(c, &a.renameOpen, func() {
		ui.Text(c, a.text("重命名分类", "Rename group")).Bold()
		submit := ui.TextInput(c, &a.renameText).Label(a.text("分类名称", "Group name")).AutoFocus().Submitted()
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, a.text("取消", "Cancel")).Clicked() {
				a.renameOpen = false
			}
			if ui.PrimaryButton(c, a.text("保存", "Save")).Clicked() {
				submit = true
			}
		})
		if submit {
			a.deferAction(func() {
				if g := a.state.Group(a.renameID); g != nil {
					if name := strings.TrimSpace(a.renameText); name != "" {
						g.Name = name
						a.changed()
					}
				}
				a.renameOpen = false
			})
		}
	})
}

func (a *App) pickFiles(directory bool) {
	if a.window == nil {
		return
	}
	groupID := a.state.ActiveGroupID
	title := a.text("选择应用或文件", "Choose applications or files")
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.window, Title: title, Multiple: true, Directory: directory})
		a.window.Update(func() {
			if err != nil {
				a.report(err)
				return
			}
			a.importPaths(paths, groupID, "")
		})
	}()
}

func (a *App) addAppsView(c *ui.Context) {
	ui.Modal(c, &a.addOpen, func() {
		ui.Text(c, a.installedAppsTitle()).FontSize(20).Bold()
		ui.TextInput(c, &a.uwpQuery).Placeholder(a.text("按名称搜索", "Filter by name")).Label(a.text("筛选应用", "Filter applications")).AutoFocus()
		var entries []model.Entry
		q := strings.ToLower(strings.TrimSpace(a.uwpQuery))
		for _, e := range a.uwp {
			if strings.Contains(strings.ToLower(e.Name), q) {
				entries = append(entries, e)
			}
		}
		if a.uwpLoading {
			ui.Text(c, a.text("正在读取应用列表…", "Loading applications…"))
		}
		if a.uwpError != "" {
			ui.Text(c, a.uwpError).TextColor(c.Theme().Danger)
		} else if !a.uwpLoading && len(entries) == 0 {
			ui.Text(c, a.text("未找到应用", "No applications found"))
		}
		a.uwpList.Key = func(i int) any { return entries[i].Path }
		ui.List(c, &a.uwpList, len(entries), func(i int) {
			e := entries[i]
			if ui.Button(c.Key(e.Path), e.Name).WidthPercent(100).Clicked() {
				a.deferAction(func() { a.addEntries(a.state.ActiveGroupID, "", []model.Entry{e}); a.addOpen = false })
			}
		}).Height(250)
		ui.Row(c).Justify(ui.End).Children(func() {
			if ui.Button(c, a.text("关闭", "Close")).Clicked() {
				a.addOpen = false
			}
		})
	})
}
