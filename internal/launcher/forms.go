package launcher

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/model"
	"my-quickstart-go/internal/platform"
)

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

func (a *App) editorView(c *ui.Context) {
	ui.Modal(c, &a.editorOpen, func() {
		title := a.text("编辑应用", "Edit application")
		if a.command {
			title = a.text("命令", "Command")
		}
		ui.Text(c, title).FontSize(20).Bold()
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
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, a.text("取消", "Cancel")).Clicked() {
				a.editorOpen = false
			}
			if ui.PrimaryButton(c, a.text("保存", "Save")).Disabled(a.command && strings.TrimSpace(a.draft.Path) == "").Clicked() {
				a.deferAction(a.saveEditor)
			}
		})
	})
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
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.window, Title: a.text("选择应用或文件", "Choose applications or files"), Multiple: true, Directory: directory})
		a.window.Update(func() {
			if err != nil {
				a.report(err)
				return
			}
			a.importPaths(paths, groupID, "")
		})
	}()
}

func (a *App) openSettings() {
	a.settingsTab = 0
	a.hotkey = a.state.Settings.ToggleHotkey
	a.settingsOpen = true
}

func (a *App) addAppsView(c *ui.Context) {
	ui.Modal(c, &a.addOpen, func() {
		ui.Text(c, a.text("添加系统应用", "Add installed application")).FontSize(20).Bold()
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

func (a *App) openAddApps() {
	a.addOpen = true
	a.uwp = nil
	a.uwpQuery = ""
	a.uwpLoading = true
	a.uwpError = ""
	if a.window == nil {
		a.uwpLoading = false
		return
	}
	go func() {
		entries, err := platform.InstalledApps()
		a.window.Update(func() {
			a.uwp = entries
			a.uwpLoading = false
			if err != nil {
				a.uwpError = err.Error()
				a.report(fmt.Errorf("%s: %w", a.text("读取系统应用失败", "Could not list applications"), err))
			}
		})
	}()
}
