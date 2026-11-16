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
