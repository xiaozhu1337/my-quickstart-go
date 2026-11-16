package scheduleui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/model"
)

func TestPythonTaskEditorAndDispatch(t *testing.T) {
	for _, language := range []string{"en", "zh-CN"} {
		for _, size := range []struct{ w, h int }{{560, 380}, {960, 620}} {
			t.Run(fmt.Sprintf("%s-%d", language, size.w), func(t *testing.T) {
				p := testPanel()
				p.Language = language
				p.edit(model.ScheduledTask{Name: "每日 Python 备份", Time: "09:30", Path: `C:\scripts\每日 backup.py`, Enabled: true})
				tt := ui.NewTester(p.View, size.w, size.h)
				label := p.tr("Python 解释器", "Python interpreter")
				for attempt := 0; attempt < 4; attempt++ {
					r, ok := tt.Find(label)
					if ok && r.Y+r.H < float32(size.h-80) {
						break
					}
					tt.Scroll(float32(size.w-60), float32(size.h-100), 0, 100)
				}
				click(t, tt, label)
				tt.Type(`"C:\Python env\python.exe"`)
				p.draft.Args = `--name "two words"`
				tt.Frame()
				if dir := os.Getenv("QUICKSTART_PYTHON_PREVIEW"); dir != "" && language == "zh-CN" {
					path := filepath.Join(dir, fmt.Sprintf("python-editor-%d.png", size.w))
					file, err := os.Create(path)
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
				if p.editOpen || len(*p.tasks) != 1 || (*p.tasks)[0].PythonPath != `C:\Python env\python.exe` {
					t.Fatal("Python task not saved", *p.tasks, p.failure)
				}
				calls := 0
				p.launch = func(task model.ScheduledTask) error {
					calls++
					if task.PythonPath != `C:\Python env\python.exe` || task.Path != `C:\scripts\每日 backup.py` || task.Args != `--name "two words"` {
						t.Fatal("Python configuration lost during launch", task)
					}
					return nil
				}
				click(t, tt, p.tr("立即执行", "Run now"))
				p.tick(time.Date(2026, 11, 16, 9, 30, 0, 0, time.Local))
				if calls != 2 {
					t.Fatal("manual and scheduled runs must both use Python", calls)
				}
				click(t, tt, p.tr("编辑", "Edit"))
				if p.draft.PythonPath != `C:\Python env\python.exe` {
					t.Fatal("editing lost the interpreter")
				}
			})
		}
	}
}
