package launcher

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/egoist/mygo/ui"

	"my-quickstart-go/internal/buildinfo"
)

func TestAboutSettings(t *testing.T) {
	previous := aboutDependencies
	t.Cleanup(func() { aboutDependencies = previous })
	aboutDependencies = []buildinfo.Dependency{
		{Path: "github.com/egoist/mygo", Version: "v0.7.1"},
		{Path: "modernc.org/sqlite", Version: "v1.60.1"},
	}
	for _, language := range []string{"en", "zh-CN"} {
		for _, width := range []int{560, 960} {
			a := fixture()
			a.state.Settings.Language = language
			tt := ui.NewTester(a.view, width, 380)
			if err := tt.Click(a.text("设置", "Settings")); err != nil {
				t.Fatal(err)
			}
			chooseSettings(t, tt, a, width, 380, a.text("关于", "About"))
			for _, text := range []string{"My Quickstart Go", buildinfo.Version, runtime.Version(), "github.com/egoist/mygo", "v0.7.1", "modernc.org/sqlite", "v1.60.1"} {
				if !tt.HasText(text) {
					t.Fatalf("%s width %d: missing %q in %q", language, width, text, tt.Texts())
				}
			}
			if err := tt.Click(a.text("关闭", "Close")); err != nil {
				t.Fatal(err)
			}
			if a.settingsOpen || !tt.HasText("Alpha App") {
				t.Fatal("closing About must return to the launcher")
			}
		}
	}
}

func TestAboutDependenciesScroll(t *testing.T) {
	previous := aboutDependencies
	t.Cleanup(func() { aboutDependencies = previous })
	aboutDependencies = nil
	for i := range 15 {
		aboutDependencies = append(aboutDependencies, buildinfo.Dependency{Path: fmt.Sprintf("example.org/library-%02d", i), Version: "v1.0.0"})
	}
	last := buildinfo.Dependency{Path: "github.com/remyoudompheng/bigfft", Version: "v0.0.0-20230129092748-24d4a6f8daec"}
	aboutDependencies = append(aboutDependencies, last)
	for _, language := range []string{"en", "zh-CN"} {
		for _, width := range []int{560, 960} {
			a := fixture()
			a.state.Settings.Language = language
			a.settingsOpen, a.settingsTab = true, 4
			tt := ui.NewTester(a.view, width, 380)
			if rect, ok := tt.Find(last.Path); !ok || rect.Y < 380 {
				t.Fatal("the full dependency list must initially extend below the viewport")
			}
			tt.Scroll(float32(width-60), 300, 0, 10000)
			for _, text := range []string{last.Path, last.Version} {
				rect, ok := tt.Find(text)
				if !ok || rect.X < 0 || rect.X+rect.W > float32(width) || rect.Y < 130 || rect.Y+rect.H > 380 {
					t.Fatalf("%s width %d: scrolled dependency %q is clipped: %+v", language, width, text, rect)
				}
			}
		}
	}
}

func TestAboutSettingsWithoutDependencyInfo(t *testing.T) {
	previous := aboutDependencies
	t.Cleanup(func() { aboutDependencies = previous })
	aboutDependencies = nil
	a := fixture()
	a.settingsOpen, a.settingsTab = true, 4
	tt := ui.NewTester(a.view, 560, 380)
	if !tt.HasText(buildinfo.Version) || !tt.HasText("Dependency version information is unavailable in this build.") {
		t.Fatalf("missing build info must keep the About page usable: %q", tt.Texts())
	}
}

func (a *App) aboutSettings(c *ui.Context) {
	a.settingsSection(c, "My Quickstart Go", "", func() {
		a.settingRow(c, a.text("程序版本", "Application version"), "", func() {
			ui.Text(c, buildinfo.Version)
		})
		a.settingRow(c, a.text("Go 版本", "Go version"), "", func() {
			ui.Text(c, runtime.Version())
		})
		a.settingRow(c, a.text("运行平台", "Platform"), "", func() {
			ui.Text(c, runtime.GOOS+" / "+runtime.GOARCH)
		})
	})
	a.settingsSection(c, a.text("依赖库版本", "Dependency versions"), a.text("当前程序实际使用的依赖库。", "Libraries included in this build."), func() {
		if len(aboutDependencies) == 0 {
			ui.Text(c, a.text("此构建未提供依赖版本信息。", "Dependency version information is unavailable in this build.")).TextColor(c.Theme().TextMuted)
		}
		for _, dep := range aboutDependencies {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, dep.Path).FontSize(11).Grow(1).SingleLine().Tooltip(dep.Path)
				version := dep.Version
				if version == "" || version == "(devel)" {
					version = a.text("开发版本", "Development build")
				}
				ui.Text(c, version).FontSize(11)
			})
		}
	})
}
