package model

import (
	"testing"
	"time"
	_ "time/tzdata"
)

func TestDailyTimeValidation(t *testing.T) {
	for _, value := range []string{"00:00", "09:30", "23:59"} {
		if _, err := ParseDailyTime(value); err != nil {
			t.Fatal(value, err)
		}
	}
	for _, value := range []string{"", "9:30", "24:00", "12:60", "12:00:00", " 09:30", "ab:cd"} {
		if _, err := ParseDailyTime(value); err == nil {
			t.Fatalf("accepted invalid time %q", value)
		}
	}
}

func TestPythonTaskValidation(t *testing.T) {
	task := ScheduledTask{ID: "python", Name: "Python", Time: "09:00", Path: "job.py"}
	for _, path := range []string{"job.py", "job.PYW", `"C:\my tasks\job.py"`} {
		task.Path = path
		if !task.IsPython() || task.Validate() != nil {
			t.Fatal("valid Python script rejected", task)
		}
	}
	for _, interpreter := range []string{"python\x00.exe", "python\n.exe", "python\r.exe"} {
		task.PythonPath = interpreter
		if task.Validate() == nil {
			t.Fatal("invalid interpreter accepted")
		}
	}
}

func TestDailyTaskClockAndRestart(t *testing.T) {
	zone := time.FixedZone("local", 8*60*60)
	now := time.Date(2026, 11, 16, 9, 30, 40, 0, zone)
	task := ScheduledTask{ID: "task", Name: "Daily", Path: "app.exe", Time: "09:30", Enabled: true}
	if err := task.Validate(); err != nil {
		t.Fatal(err)
	}
	if !task.Due(now) || task.Due(now.Add(-time.Minute)) || task.Due(now.Add(time.Minute)) {
		t.Fatal("only the configured local minute should be due")
	}
	if got := task.NextRun(now); !got.Equal(now.Truncate(time.Minute)) {
		t.Fatal("current minute should be next", got)
	}
	task.LastScheduledDate = "2026-11-16"
	if task.Due(now) || task.Due(now.AddDate(0, 0, -1)) || !task.Due(now.AddDate(0, 0, 1)) {
		t.Fatal("persisted dates must prevent repeats, including clock rollback")
	}
	if got := task.NextRun(now); got.Format("2006-01-02 15:04") != "2026-11-17 09:30" {
		t.Fatal("next run must be tomorrow", got)
	}
	task.Enabled = false
	if task.Due(now.AddDate(0, 0, 1)) || !task.NextRun(now).IsZero() {
		t.Fatal("disabled tasks cannot run")
	}
}

func TestNextRunMidnightAndDaylightSaving(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ now, clock, want string }{
		{"2026-12-31 23:59:40", "00:00", "2027-01-01 00:00"},
		{"2026-03-08 01:59:00", "02:30", "2026-03-09 02:30"},
		{"2026-11-16 09:31:00", "09:30", "2026-11-17 09:30"},
	} {
		now, err := time.ParseInLocation("2006-01-02 15:04:05", tc.now, zone)
		if err != nil {
			t.Fatal(err)
		}
		task := ScheduledTask{Time: tc.clock, Enabled: true}
		if got := task.NextRun(now).Format("2006-01-02 15:04"); got != tc.want {
			t.Fatalf("%s next = %s, want %s", tc.now, got, tc.want)
		}
	}
}
