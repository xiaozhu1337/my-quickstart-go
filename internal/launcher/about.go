package launcher

import (
	"runtime"

	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/buildinfo"
)

var aboutDependencies = buildinfo.Dependencies()

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
