package launcher

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/model"
)

func TestScheduledTabAndSettingsNavigation(t *testing.T) {
	for _, language := range []string{"en", "zh-CN"} {
		for _, size := range []struct{ w, h int }{{560, 380}, {960, 620}} {
			a := fixture()
			a.state.Settings.Language = language
			yesterday := time.Now().AddDate(0, 0, -1)
			lastRun := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 18, 0, 0, 0, time.Local)
			a.state.Tasks = []model.ScheduledTask{
				{ID: "backup", Name: "每日备份", Time: "21:30", Path: `C:\Tools\backup.exe`, Args: `--daily`, Enabled: true},
				{ID: "report", Name: "生成日报", Time: "18:00", Path: `C:\Tools\report.cmd`, Enabled: true, LastScheduledDate: lastRun.Format(time.DateOnly), LastRunAt: lastRun.UnixMilli(), LastStatus: "started"},
				{ID: "disabled", Name: "清理临时目录", Time: "09:00", Path: `C:\Tools\cleanup.exe`},
			}
			tt := ui.NewTester(a.view, size.w, size.h)
			for _, label := range []string{a.text("定时任务", "Scheduled tasks"), a.text("设置", "Settings")} {
				rect, ok := tt.Find(label)
				if !ok || rect.X < 0 || rect.X+rect.W > float32(size.w) || rect.Y > 50 {
					t.Fatalf("navigation is clipped at %d: %s %+v", size.w, label, rect)
				}
			}
			if err := tt.Click(a.text("定时任务", "Scheduled tasks")); err != nil {
				t.Fatal(err)
			}
			if a.mainTab != 2 || !tt.HasText("每日备份") || tt.HasText("Alpha App") {
				t.Fatal("scheduled tab is missing")
			}
			tt.Key(ui.Cmd, ui.KeyF)
			if a.searchOpen {
				t.Fatal("launcher search leaked into scheduled tasks")
			}
			if err := tt.Click(a.text("设置", "Settings")); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click(a.text("定时任务", "Scheduled tasks")); err != nil {
				t.Fatal(err)
			}
			if a.settingsOpen || a.mainTab != 2 || !tt.HasText("每日备份") {
				t.Fatal("settings did not return to the scheduled tab")
			}
			a.message = "Could not save scheduled tasks"
			tt.Frame()
			if rect, ok := tt.Find(a.message); !ok || rect.Y+rect.H > float32(size.h) {
				t.Fatal("save failure must remain visible in scheduled tasks", rect)
			}
			a.message = ""
			tt.Frame()
			if dir := os.Getenv("QUICKSTART_SCHEDULE_PREVIEW"); dir != "" && language == "zh-CN" {
				name := "schedule.png"
				if size.w == 560 {
					name = "schedule-small.png"
				}
				file, err := os.Create(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(file, tt.Image())
				file.Close()
				if err != nil {
					t.Fatal(err)
				}
				if err := tt.Click("新增任务"); err != nil {
					t.Fatal(err)
				}
				file, err = os.Create(filepath.Join(dir, "editor-"+name))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(file, tt.Image())
				file.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
