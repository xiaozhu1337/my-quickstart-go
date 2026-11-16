package scheduleui

import (
	"log"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/model"
	"my-quickstart-go/internal/platform"
)

func (p *Panel) edit(task model.ScheduledTask) {
	p.draft, p.failure, p.editOpen = task, "", true
}

func (p *Panel) save() {
	task := p.draft
	task.Name, task.Time = strings.TrimSpace(task.Name), strings.TrimSpace(task.Time)
	task.Path = strings.Trim(strings.TrimSpace(task.Path), "\"")
	if task.Name == "" || task.Path == "" {
		p.failure = p.tr("请填写任务名称和程序路径。", "Enter a task name and program path.")
		return
	}
	if _, err := model.ParseDailyTime(task.Time); err != nil {
		p.failure = p.tr("请输入有效的时间，格式为 HH:MM，例如 09:30（00:00–23:59）。", "Enter a valid HH:MM time, e.g. 09:30 (00:00-23:59).")
		return
	}
	if task.ID == "" {
		task.ID = model.ID()
	}
	if err := task.Validate(); err != nil {
		p.failure = p.tr("程序路径和参数不能包含换行或空字符。", "Program path and arguments cannot contain newlines or NUL.")
		return
	}
	if old := p.task(task.ID); old != nil {
		if p.running[task.ID] {
			return
		}
		// A timer may have fired while this editor was open; keep its latest result.
		task.LastRunAt, task.LastStatus, task.LastError = old.LastRunAt, old.LastStatus, old.LastError
		task.LastScheduledDate = old.LastScheduledDate
		*old = task
	} else {
		*p.tasks = append(*p.tasks, task)
	}
	p.editOpen, p.failure = false, ""
	p.changed()
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
				ui.Text(c, p.tr("每天执行时间（24 小时制）", "Daily time (24-hour clock)"))
				ui.TextInput(c, &p.draft.Time).Label(p.tr("执行时间", "Time")).Placeholder("09:30")
				ui.Text(c, p.tr("程序路径", "Program path"))
				ui.Row(c).Gap(8).Children(func() {
					ui.TextInput(c, &p.draft.Path).Label(p.tr("程序路径", "Program path")).Grow(1)
					if ui.Button(c, p.tr("浏览…", "Browse…")).Disabled(p.picking).Clicked() {
						p.pickProgram()
					}
				})
				ui.Text(c, p.tr("启动参数（可选）", "Arguments (optional)"))
				ui.TextInput(c, &p.draft.Args).Label(p.tr("启动参数", "Arguments"))
				ui.Checkbox(c, &p.draft.Enabled, p.tr("启用此任务", "Enable this task"))
				ui.Text(c, p.tr("支持程序及 .bat / .cmd 脚本；脚本后台运行，不弹出控制台窗口。", "Supports programs and .bat / .cmd scripts. Scripts run without a console window.")).FontSize(11).TextColor(c.Theme().TextMuted)
			})
		})
		if p.failure != "" {
			ui.Text(c, p.failure).TextColor(c.Theme().Danger).MaxHeight(48)
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, p.tr("取消", "Cancel")).Disabled(p.picking).Clicked() {
				p.editOpen = false
			}
			if ui.PrimaryButton(c, p.tr("保存", "Save")).Disabled(p.picking || p.running[p.draft.ID]).Clicked() {
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
			p.changed()
			break
		}
	}
	p.removing = false
}

func (p *Panel) pickProgram() {
	if p.Window == nil {
		return
	}
	p.picking = true
	title := p.tr("选择程序或脚本", "Choose a program or script")
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: p.Window, Title: title})
		var entry model.Entry
		if err == nil && len(paths) > 0 {
			entry, err = platform.Resolve(paths[0], false)
		}
		p.Window.Update(func() {
			p.picking = false
			if err != nil {
				p.failure = err.Error()
				log.Printf("choose scheduled program: %v", err)
			} else if p.editOpen && entry.Path != "" {
				p.draft.Path, p.draft.Args = entry.Path, entry.Args
				if strings.TrimSpace(p.draft.Name) == "" {
					p.draft.Name = entry.Name
				}
			}
		})
	}()
}
