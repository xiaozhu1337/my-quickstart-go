package scheduleui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/model"
	"my-quickstart-go/internal/platform"
)

func TestCommandTaskEditorAndDispatch(t *testing.T) {
	for _, language := range []string{"en", "zh-CN"} {
		for _, size := range []struct{ w, h int }{{560, 380}, {960, 620}} {
			t.Run(fmt.Sprintf("%s-%d", language, size.w), func(t *testing.T) {
				p := testPanel()
				p.Language = language
				p.edit(model.ScheduledTask{Name: "每日 Python 备份", Time: "09:30", Enabled: true})
				tt := ui.NewTester(p.View, size.w, size.h)
				command := `"C:\Python env\python.exe" "C:\scripts\每日 backup.py" --name "two words"`
				click(t, tt, p.tr("执行命令", "Command"))
				tt.Type(command)
				if dir := os.Getenv("QUICKSTART_COMMAND_PREVIEW"); dir != "" && language == "zh-CN" {
					file, err := os.Create(filepath.Join(dir, fmt.Sprintf("command-editor-%d.png", size.w)))
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(file, tt.Image())
					file.Close()
					if err != nil {
						t.Fatal(err)
					}
				}
				click(t, tt, p.tr("保存", "Save"))
				if p.editOpen || len(*p.tasks) != 1 || (*p.tasks)[0].Command != command || (*p.tasks)[0].Path != "" {
					t.Fatal("command task not saved", *p.tasks, p.failure)
				}
				calls := 0
				p.launch = func(task model.ScheduledTask) error {
					calls++
					if task.Command != command {
						t.Fatal("command changed during launch", task)
					}
					return nil
				}
				click(t, tt, p.tr("立即执行", "Run now"))
				p.tick(time.Date(2026, 11, 16, 9, 30, 0, 0, time.Local))
				if calls != 2 {
					t.Fatal("manual and scheduled runs must both use the complete command", calls)
				}
				click(t, tt, p.tr("编辑", "Edit"))
				if p.draft.Command != command {
					t.Fatal("editing lost the command")
				}
			})
		}
	}
}

func TestLegacyCommandEditing(t *testing.T) {
	p := testPanel()
	old := model.ScheduledTask{ID: "old", Name: "Old Python", Time: "09:30", Path: "task.py", PythonPath: "venv/python.exe", Args: `--name "two words"`, Enabled: true, LastScheduledDate: "2026-11-15"}
	*p.tasks = append(*p.tasks, old)
	p.edit(old)
	if p.draft.Command != platform.ScheduledCommand(old) || p.draft.Command == "" {
		t.Fatal("legacy fields were not combined")
	}
	p.save()
	if (*p.tasks)[0] != old {
		t.Fatal("unchanged legacy command must retain original launch behavior", *p.tasks)
	}
	p.edit(old)
	p.draft.Command = `python C:\xx.py`
	p.save()
	got := (*p.tasks)[0]
	if got.Command != `python C:\xx.py` || got.Path != "" || got.Args != "" || got.PythonPath != "" || got.LastScheduledDate != old.LastScheduledDate {
		t.Fatal("changed command must replace legacy fields and preserve history", got)
	}
}

func TestResetViewPreservesScheduledWork(t *testing.T) {
	p := testPanel()
	*p.tasks = []model.ScheduledTask{{ID: "saved", Name: "Saved", Time: "09:30", Command: "backup.exe", Enabled: true}}
	want := append([]model.ScheduledTask(nil), (*p.tasks)...)
	p.edit(model.ScheduledTask{Name: "Unsaved", Command: "unsaved.exe"})
	p.query, p.page, p.failure, p.removing, p.removeID = "missing", 2, "error", true, "saved"
	p.running["saved"] = true
	p.pending = append(p.pending, func() { t.Fatal("stale UI action executed") })
	p.ResetView()
	if p.ModalOpen() || p.draft != (model.ScheduledTask{}) || p.query != "" || p.page != 0 || p.failure != "" || p.removeID != "" || len(p.pending) != 0 {
		t.Fatal("reset did not clear transient UI state")
	}
	if !p.Busy() || p.stopped || !reflect.DeepEqual(want, *p.tasks) {
		t.Fatal("reset changed scheduler state or saved tasks")
	}
	p.finish("saved", nil)
	calls := 0
	p.launch = func(model.ScheduledTask) error { calls++; return nil }
	p.tick(time.Date(2026, 11, 16, 9, 30, 0, 0, time.Local))
	if calls != 1 {
		t.Fatal("reset stopped future scheduled launches")
	}
}
