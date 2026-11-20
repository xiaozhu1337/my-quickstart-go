package scheduleui

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/model"
	"my-quickstart-go/internal/platform"
)

func (p *Panel) edit(task model.ScheduledTask) {
	task.Command = platform.ScheduledCommand(task)
	p.draft, p.failure, p.editOpen = task, "", true
}

func (p *Panel) save() {
	task := p.draft
	task.Name, task.Time = strings.TrimSpace(task.Name), strings.TrimSpace(task.Time)
	task.Command = strings.TrimSpace(task.Command)
	if task.Name == "" || task.Command == "" {
		p.failure = p.tr("请填写任务名称和执行命令。", "Enter a task name and command.")
		return
	}
	if _, err := model.ParseDailyTime(task.Time); !task.AtBoot && err != nil {
		p.failure = p.tr("请输入有效的时间，格式为 HH:MM，例如 09:30（00:00–23:59）。", "Enter a valid HH:MM time, e.g. 09:30 (00:00-23:59).")
		return
	}
	if task.ID == "" {
		task.ID = model.ID()
	}
	// Editing only a name/time must preserve old interpreter lookup and working directory.
	if old := p.task(task.ID); old != nil && task.Command == platform.ScheduledCommand(*old) {
		task.Command, task.Path, task.Args, task.PythonPath = old.Command, old.Path, old.Args, old.PythonPath
	} else {
		task.Path, task.Args, task.PythonPath = "", "", ""
	}
	if err := task.Validate(); err != nil {
		p.failure = p.tr("执行命令不能包含换行或空字符。", "The command cannot contain newlines or NUL.")
		return
	}
	if old := p.task(task.ID); old != nil {
		if p.running[task.ID] {
			return
		}
		// A timer may have fired while this editor was open; keep its latest result.
		task.LastRunAt, task.LastStatus, task.LastError = old.LastRunAt, old.LastStatus, old.LastError
		task.LastScheduledDate, task.LastBootID = old.LastScheduledDate, old.LastBootID
		*old = task
	} else {
		*p.tasks = append(*p.tasks, task)
	}
	p.editOpen, p.failure = false, ""
	p.scheduleChanged()
}

func (p *Panel) dialogs(c *ui.Context) {
	width, height := c.Size()
	title := p.tr("新增定时任务", "New scheduled task")
	if p.draft.ID != "" {
		title = p.tr("编辑定时任务", "Edit scheduled task")
	}
	ui.Modal(c, &p.editOpen, func() {
		ui.Text(c, title).FontSize(20).Bold()
		ui.Scroll(c).FillWidth().Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, p.tr("任务名称", "Task name"))
				ui.TextInput(c, &p.draft.Name).Label(p.tr("任务名称", "Task name")).AutoFocus()
				if runtime.GOOS == "windows" {
					ui.Checkbox(c, &p.draft.AtBoot, p.tr("开机时执行（登录 Windows 后）", "Run at startup (after Windows sign-in)"))
				}
				if !p.draft.AtBoot {
					ui.Text(c, p.tr("每天执行时间（24 小时制）", "Daily time (24-hour clock)"))
					ui.TextInput(c, &p.draft.Time).Label(p.tr("执行时间", "Time")).Placeholder("09:30")
				}
				ui.Text(c, p.tr("执行命令（含参数）", "Command (including arguments)"))
				ui.TextInput(c, &p.draft.Command).Label(p.tr("执行命令", "Command")).Placeholder(`python C:\scripts\task.py`)
				ui.Text(c, p.tr("直接输入完整命令，含空格的路径请加双引号。", "Enter the complete command. Put paths containing spaces in double quotes.")).FontSize(11).TextColor(c.Theme().TextMuted)
				ui.Checkbox(c, &p.draft.Enabled, p.tr("启用此任务", "Enable this task"))
				if runtime.GOOS == "windows" {
					hidden, shown, legacy := p.tr("隐藏窗口", "Hide window"), p.tr("显示窗口", "Show window"), p.tr("仅隐藏控制台", "Hide console only")
					mode := map[string]string{model.WindowHidden: hidden, model.WindowShown: shown, "": legacy}[p.draft.WindowMode]
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Text(c, p.tr("启动程序时", "Program window"))
						if ui.Select(c, &mode, []string{hidden, shown, legacy}).Label(p.tr("窗口显示方式", "Window visibility")).Width(160).Changed() {
							p.draft.WindowMode = map[string]string{hidden: model.WindowHidden, shown: model.WindowShown, legacy: ""}[mode]
						}
					})
				}
			})
		})
		if p.failure != "" {
			ui.Text(c, p.failure).TextColor(c.Theme().Danger).MaxHeight(48)
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, p.tr("取消", "Cancel")).Clicked() {
				p.editOpen = false
			}
			if ui.PrimaryButton(c, p.tr("保存", "Save")).Disabled(p.running[p.draft.ID]).Clicked() {
				p.pending = append(p.pending, p.save)
			}
		})
	}).Label(title).Width(min(620, width-32)).Height(min(500, height-32))
	ui.Modal(c, &p.removing, func() {
		ui.Text(c, p.tr("删除定时任务", "Delete scheduled task")).Bold()
		if task := p.task(p.removeID); task != nil {
			ui.Text(c, p.tr("确定删除任务“", "Delete task \"")+task.Name+p.tr("”？程序文件会保留。", "\"? The program file will be kept."))
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, p.tr("取消", "Cancel")).Clicked() {
				p.removing = false
			}
			if ui.Button(c, p.tr("确认删除", "Confirm delete")).Disabled(p.running[p.removeID]).Clicked() {
				p.pending = append(p.pending, p.remove)
			}
		})
	}).Width(min(460, width-32))
}

func (p *Panel) remove() {
	if p.running[p.removeID] {
		return
	}
	for i, task := range *p.tasks {
		if task.ID == p.removeID {
			*p.tasks = append((*p.tasks)[:i], (*p.tasks)[i+1:]...)
			p.scheduleChanged()
			break
		}
	}
	p.removing = false
}
