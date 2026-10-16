package launcher

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *App) settingsView(c *ui.Context) {
	ui.Modal(c, &a.settingsOpen, func() {
		ui.Text(c, a.text("设置", "Settings")).FontSize(20).Bold()
		ui.Tabs(c, &a.settingsTab, a.text("外观", "Appearance"), a.text("布局", "Layout"), a.text("行为", "Behavior"), a.text("热键", "Hotkey"))
		ui.Scroll(c).Height(280).Children(func() {
			ui.Column(c).Gap(12).Padding(4).Children(func() {
				s := &a.state.Settings
				switch a.settingsTab {
				case 0:
					ui.Text(c, a.text("语言", "Language"))
					if ui.Select(c, &s.Language, []string{"zh-CN", "en"}).Label(a.text("语言", "Language")).Changed() {
						mygo.App.SetLanguages(s.Language)
						a.rebuildTray()
						a.changed()
					}
					ui.Text(c, a.text("主题", "Theme"))
					if ui.Select(c, &s.Theme, []string{"dark", "light"}).Label(a.text("主题", "Theme")).Changed() {
						a.changed()
					}
					ui.Text(c, a.text("字体", "Font"))
					fonts := append([]string{"maye", "system", "segoe", "inter", "mono"}, a.fonts...)
					if ui.Autocomplete(c, &s.FontFamily, fonts).Label(a.text("字体", "Font")).Changed() {
						a.changed()
					}
					a.settingNumber(c, a.text("界面字号", "UI font size"), &s.FontSize, 10, 22, 1)
				case 1:
					a.settingNumber(c, a.text("分类栏宽度", "Sidebar width"), &s.SidebarWidth, 90, 320, 2)
					a.settingNumber(c, a.text("卡片宽度", "Card width"), &s.CardWidth, 50, 480, 2)
					a.settingNumber(c, a.text("卡片高度", "Card height"), &s.CardHeight, 40, 360, 2)
					a.settingNumber(c, a.text("卡片字号", "Card font size"), &s.CardFontSize, 9, 18, 1)
					a.settingNumber(c, a.text("图标大小", "Icon size"), &s.CardIconScale, 16, 128, 2)
				case 2:
					if ui.Checkbox(c, &s.DblClickBlankToHide, a.text("双击卡片区域空白处隐藏", "Hide on double click in empty grid space")).Changed() {
						a.changed()
					}
					if ui.Checkbox(c, &s.AlwaysOnTop, a.text("窗口置顶", "Always on top")).Changed() {
						if a.window != nil {
							a.window.SetAlwaysOnTop(s.AlwaysOnTop)
						}
						a.changed()
					}
					if ui.Checkbox(c, &s.HideOnStartup, a.text("启动时隐藏窗口", "Hide window when the application starts")).Changed() {
						a.changed()
					}
					if ui.Checkbox(c, &s.UseRelativePath, a.text("新增文件使用相对路径", "Use relative paths for newly added files")).Changed() {
						a.changed()
					}
					ui.Text(c, a.text("相对路径以启动器程序所在目录为基准。", "Relative paths are based on the launcher executable directory.")).FontSize(11).TextColor(c.Theme().TextMuted)
					if ui.Checkbox(c, &s.RememberWindowBounds, a.text("记住窗口位置和大小", "Remember window position and size")).Changed() {
						a.captureBounds()
						a.changed()
					}
				case 3:
					ui.Text(c, a.text("全局显示 / 隐藏快捷键", "Global show / hide shortcut"))
					ui.TextInput(c, &a.hotkey).Label(a.text("全局快捷键", "Global shortcut")).Placeholder("Ctrl+Alt+Space")
					ui.Text(c, a.text("留空后应用可注销快捷键。", "Apply an empty value to unregister the shortcut.")).FontSize(11).TextColor(c.Theme().TextMuted)
					if ui.PrimaryButton(c, a.text("应用快捷键", "Apply shortcut")).Clicked() {
						if err := a.applyHotkey(a.hotkey); err != nil {
							a.report(err)
						} else {
							s.ToggleHotkey = a.registeredHotkey
							a.changed()
							a.message = a.text("快捷键已更新", "Shortcut updated")
						}
					}
				}
			})
		})
		ui.Row(c).Justify(ui.End).Children(func() {
			if ui.Button(c, a.text("关闭", "Close")).Clicked() {
				a.settingsOpen = false
			}
		})
	})
}

func (a *App) settingNumber(c *ui.Context, label string, value *float64, min, max, step float64) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).Grow(1)
		if ui.NumberInput(c, value, min, max, step).Label(label).Width(110).Changed() {
			a.sidebar = float32(a.state.Settings.SidebarWidth)
			a.changed()
		}
	})
}
