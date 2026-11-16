package scheduleui

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/model"
)

func testPanel() *Panel {
	tasks := []model.ScheduledTask{}
	p := New(&tasks, func() {}, nil)
	p.Language = "en"
	return p
}

func click(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
}

func TestTaskEditorAndListActions(t *testing.T) {
	for _, width := range []int{560, 960} {
		p := testPanel()
		tt := ui.NewTester(p.View, width, 340)
		click(t, tt, "Add task")
		click(t, tt, "Save")
		if !p.editOpen || p.failure == "" {
			t.Fatal("empty task must stay in editor")
		}
		click(t, tt, "Task name")
		tt.Type("Daily backup")
		click(t, tt, "Time")
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type("25:00")
		p.draft.Command = "backup.exe --daily"
		click(t, tt, "Save")
		if !p.editOpen || len(*p.tasks) != 0 {
			t.Fatal("invalid time must not save")
		}
		click(t, tt, "Time")
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type("21:30")
		click(t, tt, "Save")
		if p.editOpen || len(*p.tasks) != 1 || !tt.HasText("Daily backup") {
			t.Fatal("task not saved", p.failure, tt.Texts())
		}
		click(t, tt, "Disable")
		if (*p.tasks)[0].Enabled {
			t.Fatal("disable failed")
		}
		click(t, tt, "Enable")
		calls := 0
		p.launch = func(entry model.ScheduledTask) error {
			calls++
			if entry.Command != "backup.exe --daily" {
				t.Fatal("wrong program", entry)
			}
			return nil
		}
		click(t, tt, "Run now")
		if calls != 1 || (*p.tasks)[0].LastScheduledDate != "" || (*p.tasks)[0].LastStatus != "started" {
			t.Fatal("manual launch must not consume the daily schedule")
		}
		click(t, tt, "Edit")
		click(t, tt, "Task name")
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type("Renamed backup")
		click(t, tt, "Save")
		if (*p.tasks)[0].Name != "Renamed backup" || len(*p.tasks) != 1 || (*p.tasks)[0].LastStatus != "started" {
			t.Fatal("edit must preserve identity and history")
		}
		click(t, tt, "Delete")
		click(t, tt, "Cancel")
		if len(*p.tasks) != 1 {
			t.Fatal("cancel deleted a task")
		}
		click(t, tt, "Delete")
		click(t, tt, "Confirm delete")
		if len(*p.tasks) != 0 {
			t.Fatal("delete failed")
		}
	}
}

func TestDailyDispatchFailureAndRestart(t *testing.T) {
	p := testPanel()
	now := time.Date(2026, 11, 16, 9, 30, 10, 0, time.Local)
	*p.tasks = []model.ScheduledTask{
		{ID: "first", Name: "First", Time: "09:30", Path: "first.exe", Enabled: true},
		{ID: "second", Name: "Second", Time: "09:30", Path: "second.exe", Enabled: true},
		{ID: "off", Name: "Off", Time: "09:30", Path: "off.exe"},
	}
	var snapshot []model.ScheduledTask
	p.flush = func() error { snapshot = append([]model.ScheduledTask(nil), (*p.tasks)...); return nil }
	calls := 0
	p.launch = func(entry model.ScheduledTask) error {
		calls++
		if snapshot[calls-1].LastScheduledDate != "2026-11-16" {
			t.Fatal("launch occurred before claim persisted")
		}
		return errors.New("missing executable")
	}
	p.tick(now)
	p.tick(now.Add(time.Second))
	if calls != 2 || (*p.tasks)[0].LastStatus != "failed" || (*p.tasks)[0].LastError != "missing executable" {
		t.Fatal("all due tasks must launch once and preserve errors", calls, *p.tasks)
	}
	restarted := New(&snapshot, func() {}, nil)
	restarted.launch = func(model.ScheduledTask) error { t.Fatal("restarted task repeated"); return nil }
	restarted.tick(now.Add(20 * time.Second))
	startedTomorrow := 0
	restarted.launch = func(model.ScheduledTask) error { startedTomorrow++; return nil }
	restarted.tick(now.AddDate(0, 0, 1))
	if startedTomorrow != 2 {
		t.Fatal("tasks must run again tomorrow")
	}
	restarted.Stop()
	restarted.tick(now.AddDate(0, 0, 2))
	if startedTomorrow != 2 {
		t.Fatal("stopped scheduler launched a task")
	}
}

func TestSaveFailurePreventsProgramLaunch(t *testing.T) {
	p := testPanel()
	*p.tasks = []model.ScheduledTask{{ID: "task", Name: "Task", Time: "09:30", Path: "app.exe", Enabled: true}}
	p.flush = func() error { return errors.New("disk full") }
	p.launch = func(model.ScheduledTask) error { t.Fatal("must not launch after save failure"); return nil }
	p.tick(time.Date(2026, 11, 16, 9, 30, 0, 0, time.Local))
	if (*p.tasks)[0].LastStatus != "failed" || p.Busy() {
		t.Fatal("save failure must release busy state and remain visible")
	}
}

func TestTaskPaginationAndSearch(t *testing.T) {
	p := testPanel()
	for i := range 45 {
		*p.tasks = append(*p.tasks, model.ScheduledTask{ID: fmt.Sprint(i), Name: fmt.Sprintf("task-%02d", i), Time: "09:00", Path: "app.exe"})
	}
	tt := ui.NewTester(p.View, 960, 620)
	if !tt.HasText("45 tasks") || tt.HasText("task-20") {
		t.Fatal("first page must be bounded")
	}
	click(t, tt, "Next")
	if p.page != 1 || !tt.HasText("task-20") {
		t.Fatal("next page did not advance")
	}
	click(t, tt, "Search tasks")
	tt.Type("task-44")
	if p.page != 0 || !tt.HasText("task-44") || !tt.HasText("1 tasks") {
		t.Fatal("search must find tasks outside the current page")
	}
}
