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
			if err := tt.Click(a.text("关于", "About")); err != nil {
				t.Fatalf("%s width %d: %v", language, width, err)
			}
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
