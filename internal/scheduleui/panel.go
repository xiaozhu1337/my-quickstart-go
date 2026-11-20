package scheduleui

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/model"
	"my-quickstart-go/internal/platform"
)

// Panel state and timer callbacks are confined to the application's UI thread.
type Panel struct {
	Window      *mygo.Window
	Language    string
	tasks       *[]model.ScheduledTask
	changed     func()
	flush       func() error
	launch      func(model.ScheduledTask) error
	dispatch    func(func())
	cancelTimer chan struct{}
	filtered    []int
	filterValid bool
	filterQuery string
	stopped     bool
	running     map[string]bool
	slots       chan struct{}
	list        ui.ListState
	query       string
	page        int
	editOpen    bool
	removeID    string
	removing    bool
	draft       model.ScheduledTask
	failure     string
	pending     []func()
}

func New(tasks *[]model.ScheduledTask, changed func(), flush func() error) *Panel {
	return &Panel{tasks: tasks, changed: changed, flush: flush, launch: platform.LaunchScheduledTask,
		running: map[string]bool{}, slots: make(chan struct{}, 4)}
}

func (p *Panel) tr(zh, en string) string {
	if p.Language == "en" {
		return en
	}
	return zh
}

func (p *Panel) Busy() bool      { return len(p.running) != 0 }
func (p *Panel) ModalOpen() bool { return p.editOpen || p.removing }

// ResetView discards transient UI state without stopping schedules or active launches.
func (p *Panel) ResetView() {
	p.editOpen, p.removing = false, false
	p.draft = model.ScheduledTask{}
	p.failure, p.removeID, p.query = "", "", ""
	p.page, p.list, p.pending = 0, ui.ListState{}, nil
}

func (p *Panel) Start(window *mygo.Window) {
	if p.dispatch != nil {
		return
	}
	p.Window, p.dispatch = window, window.Update
	p.tick(time.Now())
	p.arm()
}

func (p *Panel) Stop() {
	p.stopped = true
	p.disarm()
}

func (p *Panel) disarm() {
	if p.cancelTimer != nil {
		close(p.cancelTimer)
		p.cancelTimer = nil
	}
}

func (p *Panel) arm() {
	p.disarm()
	if p.stopped || p.dispatch == nil {
		return
	}
	now := time.Now()
	var next time.Time
	for _, t := range *p.tasks {
		if p.running[t.ID] {
			continue
		}
		due := t.NextRun(now)
		if !due.IsZero() && (next.IsZero() || due.Before(next)) {
			next = due
		}
	}
	if next.IsZero() {
		return
	}
	cancel := make(chan struct{})
	p.cancelTimer = cancel
	go func() {
		waitForSchedule(now, next, cancel, func() {
			p.dispatch(func() {
				if p.cancelTimer != cancel {
					return
				}
				p.tick(time.Now())
				p.arm()
			})
		})
	}()
}

// Check the wall clock off the UI thread so clock and zone changes can reschedule
// the timer. Ordinary clock checks leave the window and task list untouched.
func waitForSchedule(start, next time.Time, cancel <-chan struct{}, wake func()) {
	_, offset := start.Zone()
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-cancel:
			return
		case <-timer.C:
			now := time.Now()
			_, zone := now.Zone()
			drift := now.Round(0).Sub(start.Round(0)) - now.Sub(start)
			remaining := next.Sub(now.Round(0))
			if remaining <= 0 || zone != offset || drift.Abs() > time.Second {
				wake()
				return
			}
			timer.Reset(min(time.Minute, remaining))
		}
	}
}

func (p *Panel) scheduleChanged() {
	p.filterValid = false
	p.changed()
	p.arm()
}

func (p *Panel) task(id string) *model.ScheduledTask {
	for i := range *p.tasks {
		if (*p.tasks)[i].ID == id {
			return &(*p.tasks)[i]
		}
	}
	return nil
}

func (p *Panel) RunAtBoot(bootID string) {
	if bootID == "" || p.stopped {
		return
	}
	now := time.Now()
	for i := range *p.tasks {
		t := &(*p.tasks)[i]
		if t.AtBoot && t.Enabled && t.LastBootID != bootID && !p.running[t.ID] {
			t.LastBootID = bootID
			p.run(t.ID, now, false)
		}
	}
}

func (p *Panel) tick(now time.Time) {
	for _, task := range *p.tasks {
		if task.Due(now) {
			p.run(task.ID, now, true)
		}
	}
}

func (p *Panel) run(id string, now time.Time, scheduled bool) {
	task := p.task(id)
	if p.stopped || task == nil || p.running[id] {
		return
	}
	p.running[id] = true
	if scheduled {
		task.LastScheduledDate = now.Format(time.DateOnly)
	}
	task.LastRunAt, task.LastStatus, task.LastError = now.UnixMilli(), "starting", ""
	snapshot := *task
	p.changed()
	work := func() error {
		// Persist the claim before starting the program, so a restart cannot repeat it.
		if p.flush != nil {
			if err := p.flush(); err != nil {
				return fmt.Errorf("save scheduled task before launch: %w", err)
			}
		}
		return p.launch(snapshot)
	}
	if p.dispatch == nil {
		p.finish(id, work())
		return
	}
	go func() {
		p.slots <- struct{}{}
		err := work()
		<-p.slots
		p.dispatch(func() { p.finish(id, err) })
	}()
}

func (p *Panel) finish(id string, err error) {
	delete(p.running, id)
	task := p.task(id)
	if task == nil {
		return
	}
	task.LastStatus, task.LastError = "started", ""
	if err != nil {
		task.LastStatus, task.LastError = "failed", err.Error()
		log.Printf("scheduled task %q: %v", task.Name, err)
	}
	p.changed()
	if p.dispatch != nil {
		p.arm()
	}
}

func (p *Panel) View(c *ui.Context) {
	ui.Column(c).FillWidth().Grow(1).Padding(16).Gap(10).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			if ui.TextInput(c, &p.query).Label(p.tr("搜索任务", "Search tasks")).Placeholder(p.tr("搜索名称或执行命令", "Search name or command")).Grow(1).Changed() {
				p.page = 0
			}
			if ui.PrimaryButton(c, p.tr("新增任务", "Add task")).Clicked() {
				p.edit(model.ScheduledTask{Time: "09:00", Enabled: true, WindowMode: model.WindowHidden})
			}
		})
		ui.Text(c, p.tr("每日任务按本地时间执行，支持托盘运行，错过的时间不补执行。", "Daily tasks run in local time, including in the tray. Missed times are skipped.")).FontSize(11).TextColor(c.Theme().TextMuted)
		p.taskList(c)
	})
	p.dialogs(c)
	actions := p.pending
	p.pending = nil
	for _, action := range actions {
		action()
	}
}

const pageSize = 20

func (p *Panel) taskList(c *ui.Context) {
	query := strings.TrimSpace(p.query)
	if !p.filterValid || p.filterQuery != query {
		q := strings.ToLower(query)
		p.filtered = p.filtered[:0]
		for i, t := range *p.tasks {
			if q == "" || strings.Contains(strings.ToLower(t.Name+" "+platform.ScheduledCommand(t)), q) {
				p.filtered = append(p.filtered, i)
			}
		}
		p.filterValid, p.filterQuery = true, query
	}
	matches := p.filtered
	pages := max(1, (len(matches)+pageSize-1)/pageSize)
	p.page = min(p.page, pages-1)
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, fmt.Sprintf(p.tr("共 %d 个任务", "%d tasks"), len(matches))).Grow(1).TextColor(c.Theme().TextMuted)
		if pages > 1 {
			if ui.Button(c, p.tr("上一页", "Previous")).Disabled(p.page == 0).Clicked() {
				p.page--
			}
			ui.Text(c, fmt.Sprintf("%d / %d", p.page+1, pages))
			if ui.Button(c, p.tr("下一页", "Next")).Disabled(p.page == pages-1).Clicked() {
				p.page++
			}
		}
	})
	if len(matches) == 0 {
		message := p.tr("暂无定时任务，点击“新增任务”设置时间和执行命令。", "No scheduled tasks. Add a task to choose a time and command.")
		if query != "" {
			message = p.tr("没有匹配的任务。", "No matching tasks.")
		}
		ui.Text(c, message).TextColor(c.Theme().TextMuted).Padding(16, 0)
	}
	visible := matches[p.page*pageSize : min(len(matches), (p.page+1)*pageSize)]
	p.list.Key = func(i int) any { return (*p.tasks)[visible[i]].ID }
	now := time.Now()
	ui.List(c, &p.list, len(visible), func(i int) {
		p.taskRow(c, (*p.tasks)[visible[i]], now)
	}).Grow(1)
}

func (p *Panel) taskRow(c *ui.Context, task model.ScheduledTask, now time.Time) {
	card := ui.Column(c.Key(task.ID)).FillWidth().Padding(12).Margin(0, 0, 8, 0).Gap(6).Background(c.Theme().Surface).Radius(8)
	// Keep double-clicks on task details from reaching the page's empty-space action.
	card.DoubleClicked()
	card.Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, task.Name).Bold().SingleLine().Tooltip(task.Name).Grow(1)
			trigger := p.tr("每天 ", "Daily ") + task.Time
			if task.AtBoot {
				trigger = p.tr("开机时", "At startup")
			}
			ui.Text(c, trigger).TextColor(c.Theme().Accent)
		})
		command := platform.ScheduledCommand(task)
		ui.Text(c, command).SingleLine().Tooltip(command).FontSize(11).TextColor(c.Theme().TextMuted)
		status := p.tr("尚未执行", "Not run yet")
		if task.LastRunAt != 0 {
			result := p.tr("已启动", "Started")
			if task.LastStatus == "failed" {
				result = p.tr("启动失败：", "Failed to start: ") + task.LastError
			} else if task.LastStatus == "starting" {
				result = p.tr("上次启动结果未知", "Last launch result unknown")
				if p.running[task.ID] {
					result = p.tr("正在启动…", "Starting…")
				}
			}
			status = time.UnixMilli(task.LastRunAt).Local().Format("2006-01-02 15:04:05") + "  " + result
		}
		color := c.Theme().TextMuted
		if task.LastStatus == "failed" {
			color = c.Theme().Danger
		}
		ui.Text(c, status).SingleLine().Tooltip(status).FontSize(11).TextColor(color)
		next := p.tr("已停用", "Disabled")
		if task.AtBoot && task.Enabled {
			next = p.tr("下次开机登录时", "Next login after boot")
		} else if when := task.NextRun(now); !when.IsZero() {
			next = p.tr("下次：", "Next: ") + when.Format("2006-01-02 15:04")
		}
		ui.Text(c, next).FontSize(11).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(6).Children(func() {
			toggle := p.tr("启用", "Enable")
			if task.Enabled {
				toggle = p.tr("停用", "Disable")
			}
			if ui.Button(c, toggle).Disabled(p.running[task.ID]).Clicked() {
				p.pending = append(p.pending, func() { p.task(task.ID).Enabled = !task.Enabled; p.scheduleChanged() })
			}
			if ui.Button(c, p.tr("立即执行", "Run now")).Disabled(p.running[task.ID]).Clicked() {
				p.pending = append(p.pending, func() { p.run(task.ID, time.Now(), false) })
			}
			if ui.Button(c, p.tr("编辑", "Edit")).Disabled(p.running[task.ID]).Clicked() {
				p.edit(task)
			}
			if ui.Button(c, p.tr("删除", "Delete")).Disabled(p.running[task.ID]).Clicked() {
				p.removeID, p.removing = task.ID, true
			}
		})
	})
}
