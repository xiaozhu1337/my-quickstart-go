package launcher

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/nsgo-dev/nsgo/core"

	"my-quickstart-go/internal/model"
	w "my-quickstart-go/internal/nativeui"
	"my-quickstart-go/internal/platform"
)

func (a *App) nativeDialog(v w.Page) core.Node {
	t := a.text
	cancel := v.Button(t("取消", "Cancel"), func() { a.renameOpen, a.editorOpen, a.addOpen = false, false, false })
	if a.renameOpen {
		return v.Editor(t("重命名分类", "Rename group"), w.Field(t("分类名称", "Group name"), &a.renameText, false), w.Row(cancel, v.Button(t("保存", "Save"), func() {
			if g := a.state.Group(a.renameID); g != nil && strings.TrimSpace(a.renameText) != "" {
				g.Name = strings.TrimSpace(a.renameText)
				a.changed()
			}
			a.renameOpen = false
		})))
	}
	if a.editorOpen {
		fields := []core.Node{w.Field(t("名称", "Name"), &a.draft.Name, false)}
		if a.command {
			fields = append(fields, w.Field(t("命令内容", "Command text"), &a.draft.Path, true))
		} else {
			fields = append(fields, w.Field(t("路径", "Path"), &a.draft.Path, false), w.Field(t("参数", "Arguments"), &a.draft.Args, false), w.Field(t("环境变量（每行 KEY=value）", "Environment (one KEY=value per line)"), &a.envText, true))
		}
		title := t("编辑快捷方式", "Edit shortcut")
		if a.command && a.editID == "" {
			title = t("添加命令", "Add command")
		}
		return v.Editor(title, append(fields, w.Row(cancel, v.Primary(t("保存", "Save"), a.saveEditor)))...)
	}
	status := a.uwpError
	if a.uwpLoading {
		status = t("正在读取应用列表…", "Loading applications…")
	}
	picker := w.Column(w.Row(w.Text(a.installedAppsTitle()), cancel), w.Text(status), w.Search(v.Scope, t("按名称搜索", "Filter by name"), &a.uwpQuery, func(q string) core.Node {
		var entries []model.Entry
		for _, e := range a.uwp {
			if strings.Contains(strings.ToLower(e.Name), strings.ToLower(strings.TrimSpace(q))) {
				entries = append(entries, e)
			}
		}
		return w.List(len(entries), 1, 34, func(i int) any { return entries[i].Path }, func(i int) core.Node {
			e := entries[i]
			return v.Button(e.Name, func() { a.addEntries(a.state.ActiveGroupID, "", []model.Entry{e}); a.addOpen = false })
		})
	}))
	picker.Style.Height = core.Pt(min(460, v.Scope.Env().Window().Height-80))
	return picker
}

func (a *App) nativePick(v w.Page, dir bool) {
	group := a.state.ActiveGroupID
	finish := func(paths []string, err error) {
		if err != nil {
			a.report(err)
			v.Refresh()
			return
		}
		a.importPaths(paths, group, "")
	}
	cx := a.native.surface
	cx.Post(func() {
		if dir {
			cx.PickDirectories(core.PickDirectoriesOptions{Multiple: true}, func(dirs []core.Directory, err error) {
				var paths []string
				for _, d := range dirs {
					paths = append(paths, d.Path)
				}
				finish(paths, err)
			})
			return
		}
		cx.PickFiles(core.PickFilesOptions{Multiple: true}, func(files []core.File, err error) {
			var paths []string
			for _, f := range files {
				paths = append(paths, f.Path)
			}
			finish(paths, err)
		})
	})
}

func (a *App) newCommand() {
	a.editID = ""
	a.editGroup = a.state.ActiveGroupID
	a.draft = model.NewEntry("", "", "")
	a.envText = ""
	a.command = true
	a.editorOpen = true
}

func (a *App) openEditor(e model.Entry) {
	a.draft = e
	a.editID = e.ID
	a.editGroup = ""
	a.envText = strings.Join(e.EnvVars, "\n")
	a.command = strings.HasPrefix(strings.ToLower(strings.TrimSpace(e.Path)), "command:")
	if a.command {
		a.draft.Path = strings.TrimSpace(e.Path)[len("command:"):]
	}
	a.editorOpen = true
}

func (a *App) saveEditor() {
	e := a.draft
	if a.command {
		command := strings.TrimSpace(e.Path)
		if command == "" {
			return
		}
		e.Path = "command:" + command
		e.Args = ""
		e.EnvVars = nil
		e.Icon = ""
		e.Name = strings.TrimSpace(e.Name)
		if e.Name == "" {
			r := []rune(command)
			if len(r) > 40 {
				e.Name = string(r[:40]) + "..."
			} else {
				e.Name = command
			}
		}
	} else {
		oldGroup, old := a.state.Entry(a.editID)
		if old == nil || oldGroup == nil {
			return
		}
		e.Name = strings.TrimSpace(e.Name)
		if e.Name == "" {
			e.Name = old.Name
		}
		e.Path = strings.TrimSpace(e.Path)
		if e.Path == "" {
			e.Path = old.Path
		}
		if e.Path != old.Path {
			e.Icon = ""
		}
		e.EnvVars = nil
		for _, line := range strings.Split(a.envText, "\n") {
			line = strings.TrimSuffix(line, "\r")
			if strings.TrimSpace(line) != "" {
				e.EnvVars = append(e.EnvVars, line)
			}
		}
	}
	if a.editID == "" {
		g := a.state.Group(a.editGroup)
		if g == nil {
			return
		}
		g.Apps = append([]model.Entry{e}, g.Apps...)
	} else {
		_, old := a.state.Entry(a.editID)
		if old == nil {
			return
		}
		*old = e
	}
	a.editorOpen = false
	a.changed()
}

func (a *App) openSettings() {
	a.settingsTab = 0
	a.hotkey = a.state.Settings.ToggleHotkey
	a.settingsOpen = true
}

func (a *App) installedAppsTitle() string {
	if runtime.GOOS == "windows" {
		return a.text("添加 UWP 应用", "Add UWP application")
	}
	return a.text("添加系统应用", "Add installed application")
}

func (a *App) openAddApps() {
	a.addOpen = true
	a.uwp = nil
	a.uwpQuery = ""
	a.uwpLoading = true
	a.uwpError = ""
	if a.window == nil && a.native == nil {
		a.uwpLoading = false
		return
	}
	go func() {
		entries, err := platform.InstalledApps()
		a.post(func() {
			a.uwp = entries
			a.uwpLoading = false
			if a.native != nil && a.addOpen {
				bump(a.native.dialogContent)
			}
			if err != nil {
				a.uwpError = err.Error()
				a.report(fmt.Errorf("%s: %w", a.text("读取系统应用失败", "Could not list applications"), err))
			}
		})
	}()
}
