package scheduleui

import (
	"fmt"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"my-quickstart-go/internal/model"
	w "my-quickstart-go/internal/nativeui"
	"my-quickstart-go/internal/platform"
)

func (p *Panel) StartWithDispatch(dispatch func(func())) {
	if p.dispatch != nil {
		return
	}
	p.dispatch = dispatch
	p.tick(time.Now())
	p.arm()
}

func (p *Panel) NativeDialog(v w.Page) core.Node {
	if p.editOpen {
		enabled := core.NewSignal(v.Scope, p.draft.Enabled)
		return v.Editor(p.tr("定时任务", "Scheduled task"), w.Field(p.tr("任务名称", "Task name"), &p.draft.Name, false),
			w.Field(p.tr("每天执行时间（HH:MM）", "Daily time (HH:MM)"), &p.draft.Time, false),
			w.Field(p.tr("执行命令（含参数）", "Command (including arguments)"), &p.draft.Command, false),
			ui.Checkbox{Text: p.tr("启用此任务", "Enable this task"), Checked: enabled.Get, OnChange: func(x bool) { p.draft.Enabled = x; enabled.Set(x) }},
			w.Text(p.failure), w.Row(v.Button(p.tr("取消", "Cancel"), func() { p.editOpen = false }), v.Primary(p.tr("保存", "Save"), p.save, p.running[p.draft.ID])))
	}
	if p.removing {
		name := ""
		if task := p.task(p.removeID); task != nil {
			name = task.Name
		}
		return v.Editor(p.tr("删除定时任务", "Delete scheduled task"), w.Text(name), w.Row(
			v.Button(p.tr("取消", "Cancel"), func() { p.removing = false }), v.Button(p.tr("确认删除", "Confirm delete"), p.remove, p.running[p.removeID])))
	}
	return nil
}

func (p *Panel) Native(v w.Page) core.Node {
	return w.Column(w.Spread(v.Muted(p.tr("按本地时间每天执行，隐藏到托盘后继续运行。", "Runs daily in local time, including in the tray.")),
		v.Primary(p.tr("新增任务", "Add task"), func() { p.edit(model.ScheduledTask{Time: "09:00", Enabled: true}) })),
		w.Search(v.Scope, p.tr("搜索任务", "Search tasks"), &p.query, func(query string) core.Node {
			var matches []model.ScheduledTask
			for _, t := range *p.tasks {
				if strings.Contains(strings.ToLower(t.Name+" "+platform.ScheduledCommand(t)), strings.ToLower(strings.TrimSpace(query))) {
					matches = append(matches, t)
				}
			}
			pages := max(1, (len(matches)+pageSize-1)/pageSize)
			p.page = min(p.page, pages-1)
			nodes := []core.Node{w.Text(fmt.Sprintf(p.tr("共 %d 个任务", "%d tasks"), len(matches)))}
			if pages > 1 {
				nodes = append(nodes, w.Row(v.Button(p.tr("上一页", "Previous"), func() { p.page-- }, p.page == 0), w.Text(fmt.Sprintf("%d / %d", p.page+1, pages)), v.Button(p.tr("下一页", "Next"), func() { p.page++ }, p.page+1 == pages)))
			}
			for _, task := range matches[p.page*pageSize : min((p.page+1)*pageSize, len(matches))] {
				busy := p.running[task.ID]
				toggle := p.tr("启用", "Enable")
				if task.Enabled {
					toggle = p.tr("停用", "Disable")
				}
				status := p.lastRun(task)
				next := p.tr("已停用", "Disabled")
				if t := task.NextRun(time.Now()); !t.IsZero() {
					next = p.tr("下次 ", "Next ") + t.Format("2006-01-02 15:04")
				}
				nodes = append(nodes, v.Panel(w.Spread(w.Heading(task.Name), w.Text(task.Time)), w.Text(platform.ScheduledCommand(task)), v.Muted(status+"    "+next),
					w.Row(v.Button(toggle, func() {
						if t := p.task(task.ID); t != nil {
							t.Enabled = !t.Enabled
							p.changed()
						}
					}, busy),
						v.Button(p.tr("立即执行", "Run now"), func() { p.run(task.ID, time.Now(), false) }, busy),
						v.Button(p.tr("编辑", "Edit"), func() { p.edit(task) }, busy), v.Button(p.tr("删除", "Delete"), func() { p.removeID, p.removing = task.ID, true }, busy))))
			}
			return w.Form(nodes...)
		}))
}
