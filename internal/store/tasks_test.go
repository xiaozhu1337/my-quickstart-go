package store

import (
	"testing"
	"time"

	"my-quickstart-go/internal/model"
)

func TestScheduledTasksPersistAndRollback(t *testing.T) {
	s := openTestStore(t)
	state := sampleState()
	state.Tasks = []model.ScheduledTask{
		{ID: "daily", Name: "每日备份", Time: "23:59", Path: `C:\工具\backup.exe`, Args: `--name "中文 空格"`, Enabled: true,
			LastScheduledDate: "2026-11-16", LastRunAt: 1794830000000, LastStatus: "failed", LastError: "file not found"},
		{ID: "disabled", Name: "Disabled", Time: "00:00", Path: "other.cmd"},
		{ID: "python", Name: "Python script", Time: "12:00", Path: `C:\scripts\task.py`, PythonPath: `C:\virtual env\Scripts\python.exe`, Args: `--value "two words"`, Enabled: true},
		{ID: "command", Name: "Command", Time: "13:00", Command: `python "C:\每日 scripts\backup.py" --name "two words"`, Enabled: true},
	}
	if err := s.Save(state); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.Load()
	if err != nil {
		t.Fatal(err)
	}
	assertState(t, state, got)
	now := time.Date(2026, 11, 16, 23, 59, 30, 0, time.Local)
	if got.Tasks[0].Due(now) || !got.Tasks[0].Due(now.AddDate(0, 0, 1)) {
		t.Fatal("reopening must preserve the automatic launch claim")
	}
	if _, err := reopened.db.Exec(`CREATE TRIGGER reject_task BEFORE INSERT ON scheduled_tasks
 WHEN NEW.name = 'reject' BEGIN SELECT RAISE(ABORT, 'task failure'); END;`); err != nil {
		t.Fatal(err)
	}
	failed := state
	failed.Tasks = append([]model.ScheduledTask(nil), state.Tasks...)
	failed.Tasks[0].Name = "reject"
	failed.Settings.Theme = "light"
	if err := reopened.Save(failed); err == nil {
		t.Fatal("injected failure must abort the entire save")
	}
	got, err = reopened.Load()
	if err != nil {
		t.Fatal(err)
	}
	assertState(t, state, got)
	state.Tasks = nil
	if err := reopened.Save(state); err != nil {
		t.Fatal(err)
	}
	got, err = reopened.Load()
	if err != nil || len(got.Tasks) != 0 {
		t.Fatal("deletion did not persist", err)
	}
}

func TestTasksWithoutPythonColumnRemainReadable(t *testing.T) {
	s := openTestStore(t)
	want := sampleState()
	want.Tasks = []model.ScheduledTask{{ID: "legacy", Name: "Legacy", Path: "app.exe", Time: "09:00", Enabled: true, LastScheduledDate: "2026-11-16"}}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("ALTER TABLE scheduled_tasks DROP COLUMN python_path"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	assertState(t, want, got)
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM pragma_table_info('scheduled_tasks') WHERE name = 'python_path'").Scan(&count); err != nil || count != 0 {
		t.Fatal("loading must not modify the old schema", count, err)
	}
	want.Tasks = append(want.Tasks, model.ScheduledTask{ID: "python", Name: "Python", Path: "task.py", PythonPath: "venv/python.exe", Time: "10:00"})
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err = s.Load()
	if err != nil {
		t.Fatal(err)
	}
	assertState(t, want, got)
}

func TestTasksWithoutCommandColumnRemainReadable(t *testing.T) {
	for _, withPython := range []bool{false, true} {
		t.Run(map[bool]string{false: "original", true: "python"}[withPython], func(t *testing.T) {
			s := openTestStore(t)
			want := sampleState()
			want.Tasks = []model.ScheduledTask{{ID: "legacy", Name: "Legacy", Time: "09:00", Path: "task.py", Args: "--daily", Enabled: true, LastScheduledDate: "2026-11-16"}}
			if withPython {
				want.Tasks[0].PythonPath = "venv/python.exe"
			}
			if err := s.Save(want); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec("ALTER TABLE scheduled_tasks DROP COLUMN command"); err != nil {
				t.Fatal(err)
			}
			if !withPython {
				if _, err := s.db.Exec("ALTER TABLE scheduled_tasks DROP COLUMN python_path"); err != nil {
					t.Fatal(err)
				}
			}
			got, err := s.Load()
			if err != nil {
				t.Fatal(err)
			}
			assertState(t, want, got)
			var count int
			if err := s.db.QueryRow("SELECT count(*) FROM pragma_table_info('scheduled_tasks') WHERE name = 'command'").Scan(&count); err != nil || count != 0 {
				t.Fatal("load must not modify old schema", count, err)
			}
			want.Tasks = append(want.Tasks, model.ScheduledTask{ID: "new", Name: "New", Time: "10:00", Command: `python C:\xx.py`})
			if err := s.Save(want); err != nil {
				t.Fatal(err)
			}
			got, err = s.Load()
			if err != nil {
				t.Fatal(err)
			}
			assertState(t, want, got)
		})
	}
}

func TestUpgradeStartupTaskColumns(t *testing.T) {
	s := openTestStore(t)
	before := sampleState()
	before.Tasks = []model.ScheduledTask{{ID: "old", Name: "Old", Command: "app.exe", Time: "09:00", Enabled: true}}
	if err := s.Save(before); err != nil {
		t.Fatal(err)
	}
	for _, col := range []string{"at_boot", "window_mode", "last_boot_id"} {
		if _, err := s.db.Exec("ALTER TABLE scheduled_tasks DROP COLUMN " + col); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	assertState(t, before, got)
	after := got
	after.Tasks = append([]model.ScheduledTask(nil), got.Tasks...)
	task := &after.Tasks[0]
	task.AtBoot, task.WindowMode, task.LastBootID = true, model.WindowShown, "boot-one"
	if err := s.SaveChanges(before, after, Tasks); err != nil {
		t.Fatal(err)
	}
	got, err = s.Load()
	if err != nil {
		t.Fatal(err)
	}
	assertState(t, after, got)
}

func TestExistingDatabaseWithoutTasksRemainsReadable(t *testing.T) {
	s := openTestStore(t)
	want := sampleState()
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("DROP TABLE scheduled_tasks"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	assertState(t, want, got)
	want.Tasks = []model.ScheduledTask{{ID: "new", Name: "New", Path: "app.exe", Time: "09:00", Enabled: true}}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err = s.Load()
	if err != nil {
		t.Fatal(err)
	}
	assertState(t, want, got)
}
