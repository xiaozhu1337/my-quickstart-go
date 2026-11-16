package launcher

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

func TestFeatureTabVisibility(t *testing.T) {
	for _, language := range []string{"en", "zh-CN"} {
		for _, size := range []struct{ w, h int }{{560, 380}, {960, 620}} {
			t.Run(fmt.Sprintf("%s-%d", language, size.w), func(t *testing.T) {
				a := fixture()
				a.state.Settings.Language = language
				a.state.Tasks = []model.ScheduledTask{{ID: "kept", Name: "Kept task", Time: "23:59", Path: "job.exe", Enabled: true}}
				p := &memoryStore{}
				a.saver = &saver{store: p, wake: make(chan struct{}, 1)}
				tt := ui.NewTester(a.view, size.w, size.h)
				preview := func(name string) {
					t.Helper()
					dir := os.Getenv("QUICKSTART_TABS_PREVIEW")
					if dir == "" || language != "zh-CN" {
						return
					}
					file, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s-%d.png", name, size.w)))
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(file, tt.Image())
					file.Close()
					if err != nil {
						t.Fatal(err)
					}
				}
				click := func(label string) {
					t.Helper()
					for attempt := 0; attempt < 3; attempt++ {
						r, ok := tt.Find(label)
						if !ok || r.Y+r.H <= float32(size.h-10) {
							break
						}
						tt.Scroll(float32(size.w-40), float32(size.h-40), 0, 160)
					}
					if err := tt.Click(label); err != nil {
						t.Fatal(err)
					}
					tt.Frame()
				}
				settings := a.text("设置", "Settings")
				scheduled := a.text("定时任务", "Scheduled tasks")
				showLibrary := a.text("显示 MCP / Skills", "Show MCP / Skills")
				showSchedule := a.text("显示定时任务", "Show scheduled tasks")
				navigation := a.text("主导航", "Main navigation")
				click("MCP / Skills")
				click(settings)
				chooseSettings(t, tt, a, size.w, size.h, a.text("布局", "Layout"))
				preview("tabs-settings")
				click(showLibrary)
				if _, visible := tt.Find("MCP / Skills"); !a.state.Settings.HideLibraryTab || a.mainTab != 0 || visible {
					t.Fatal("hiding the active page must return to launcher and remove its tab")
				}
				click(scheduled)
				if a.mainTab != 2 || !tt.HasText("Kept task") {
					t.Fatal("settings return mapped the second visible tab to the wrong page")
				}
				click(a.text("应用启动", "Launcher"))
				click(scheduled)
				if a.mainTab != 2 || !tt.HasText("Kept task") {
					t.Fatal("the second visible tab must open scheduled tasks")
				}
				click(settings)
				chooseSettings(t, tt, a, size.w, size.h, a.text("布局", "Layout"))
				click(showSchedule)
				if !a.launcherOnly() || a.mainTab != 0 {
					t.Fatal("hiding all extra pages must select launcher")
				}
				if _, ok := tt.Find(navigation); ok {
					t.Fatal("launcher-only settings must not leave an empty tab bar")
				}
				if err := a.Flush(); err != nil || !p.state.Settings.HideLibraryTab || !p.state.Settings.HideScheduleTab {
					t.Fatal("visibility settings were not saved", err)
				}
				click(a.text("关闭", "Close"))
				if _, ok := tt.Find(navigation); ok || !tt.HasText("Alpha App") {
					t.Fatal("launcher-only view must show shortcuts without a tab bar")
				}
				if rect, ok := tt.Find(settings); !ok || rect.X+rect.W > a.sidebar || rect.Y+rect.H > float32(size.h) {
					t.Fatal("settings must remain accessible in the sidebar", rect)
				}
				preview("launcher-only")
				tt.Key(ui.Cmd, ui.KeyF)
				if !a.searchOpen {
					t.Fatal("launcher-only search shortcut stopped working")
				}
				tt.Key(0, ui.KeyEscape)
				click(settings)
				chooseSettings(t, tt, a, size.w, size.h, a.text("布局", "Layout"))
				click(showLibrary)
				if _, visible := tt.Find(scheduled); a.state.Settings.HideLibraryTab || !a.state.Settings.HideScheduleTab || visible {
					t.Fatal("tabs must be independently restorable")
				}
				click("MCP / Skills")
				if a.mainTab != 1 {
					t.Fatal("restored library tab opened the wrong page")
				}
				click(settings)
				chooseSettings(t, tt, a, size.w, size.h, a.text("布局", "Layout"))
				click(showSchedule)
				click(scheduled)
				if a.mainTab != 2 || !tt.HasText("Kept task") || !a.state.Tasks[0].Enabled {
					t.Fatal("showing the scheduled page must preserve enabled tasks")
				}
			})
		}
	}
}

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
