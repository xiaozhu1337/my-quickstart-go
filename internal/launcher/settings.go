package launcher

import (
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *App) settingsView(c *ui.Context) {
	width, _ := c.Size()
	labels := []string{a.text("外观", "Appearance"), a.text("布局", "Layout"), a.text("行为", "Behavior"), a.text("快捷键", "Hotkey")}
	descriptions := []string{
		a.text("让启动器更符合你的使用习惯。", "Make your launcher feel at home."),
		a.text("调整应用卡片和分类栏的显示密度。", "Find the right density for your shortcuts."),
		a.text("选择窗口、启动和文件路径的工作方式。", "Choose how the window and file paths behave."),
		a.text("随时唤起启动器，保持键盘操作流畅。", "Keep your launcher one keystroke away."),
	}
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(18, 20).Gap(16).Children(func() {
			if ui.Button(c, a.text("返回启动器", "Back to launcher")).Label(a.text("关闭", "Close")).Height(36).Clicked() {
				a.settingsOpen = false
			}
			ui.Text(c, a.text("设置", "Settings")).FontSize(20).Bold().Grow(1)
			ui.Text(c, a.text("自动保存", "Saved automatically")).FontSize(11).TextColor(c.Theme().TextMuted)
		})
		ui.Divider(c)
		if width < 760 {
			ui.Tabs(c, &a.settingsTab, labels...).Padding(12, 20, 0, 20)
		}
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			if width >= 760 {
				ui.Column(c).Width(176).Padding(16, 12).Gap(6).Background(c.Theme().Surface.Alpha(0.5)).Children(func() {
					ui.Text(c, a.text("偏好设置", "PREFERENCES")).FontSize(10).Bold().TextColor(c.Theme().TextMuted).Padding(8)
					for i, label := range labels {
						button := ui.Button(c.Key(label), label).Height(40).Label(label)
						if a.settingsTab == i {
							button.Background(c.Theme().Accent.Alpha(0.14)).Border(1, c.Theme().Accent.Alpha(0.25))
						}
						if button.Clicked() {
							a.settingsTab = i
						}
					}
				})
			}
			ui.Scroll(c.Key(a.settingsTab)).Grow(1).Children(func() {
				ui.Column(c).Padding(22, 24).Gap(18).Children(func() {
					ui.Column(c).Gap(6).Children(func() {
						ui.Text(c, labels[a.settingsTab]).FontSize(24).Bold()
						ui.Text(c, descriptions[a.settingsTab]).TextColor(c.Theme().TextMuted)
					})
					switch a.settingsTab {
					case 0:
						a.appearanceSettings(c)
					case 1:
						a.layoutSettings(c)
					case 2:
						a.behaviorSettings(c)
					case 3:
						a.hotkeySettings(c)
					}
				})
			})
		})
		ui.Divider(c)
		a.statusView(c)
	})
}

func (a *App) appearanceSettings(c *ui.Context) {
	s := &a.state.Settings
	a.settingsSection(c, a.text("配色主题", "Color theme"), a.text("点击预览，选择会立即应用到整个界面。", "Choose a palette to apply it throughout the app."), func() {
		width, _ := c.Size()
		columns := 5
		if width < 900 {
			columns = 3
		}
		ui.Grid(c).Columns(columns).Gap(10).Children(func() {
			for _, theme := range themes {
				p := palette(theme.id)
				name := a.text(theme.zh, theme.en)
				button := ui.ButtonBase(c.Key(theme.id)).Label(name).Padding(6).Radius(9).Border(1, c.Theme().Border).Background(c.Theme().Background)
				selected := s.Theme == theme.id
				if selected {
					button.Border(2, c.Theme().Accent)
				}
				button.Children(func() {
					ui.Column(c).FillWidth().Gap(8).Children(func() {
						ui.Row(c).Height(46).Padding(7).Gap(5).Radius(5).Background(p.Background).Children(func() {
							ui.Box(c).Width(12).HeightPercent(100).Radius(3).Background(p.Accent.Alpha(0.5))
							ui.Column(c).Grow(1).Gap(4).Children(func() {
								ui.Box(c).Height(5).WidthPercent(70).Radius(2).Background(p.TextMuted)
								ui.Row(c).Grow(1).Gap(4).Children(func() {
									ui.Box(c).Grow(1).HeightPercent(100).Radius(3).Background(p.Surface).Border(1, p.Border)
									ui.Box(c).Grow(1).HeightPercent(100).Radius(3).Background(p.Accent)
								})
							})
						})
						ui.Text(c, name).FontSize(11).SingleLine().AlignSelf(ui.Center)
					})
				})
				if button.Clicked() {
					s.Theme = theme.id
					if a.window != nil {
						syncNativeTheme(theme.id)
					}
					a.changed()
				}
			}
		})
	})
	a.settingsSection(c, a.text("语言与文字", "Language & text"), "", func() {
		a.settingRow(c, a.text("语言", "Language"), a.text("界面和托盘菜单的显示语言。", "Language used in the app and tray menu."), func() {
			if ui.Select(c, &s.Language, []string{"zh-CN", "en"}).Label(a.text("语言", "Language")).Width(160).Changed() {
				mygo.App.SetLanguages(s.Language)
				a.rebuildTray()
				a.changed()
			}
		})
		a.settingRow(c, a.text("字体", "Font"), a.text("选择系统字体或输入字体名称。", "Choose a system font or enter a family name."), func() {
			fonts := append([]string{"maye", "system", "segoe", "inter", "mono"}, a.fonts...)
			if ui.Autocomplete(c, &s.FontFamily, fonts).Label(a.text("字体", "Font")).Width(160).Changed() {
				a.changed()
			}
		})
		a.settingNumber(c, a.text("界面字号", "UI font size"), &s.FontSize, 10, 22, 1)
	})
}

func (a *App) layoutSettings(c *ui.Context) {
	s := &a.state.Settings
	a.settingsSection(c, a.text("应用卡片", "Shortcut cards"), a.text("尺寸以设备无关像素计算。", "Sizes use device-independent pixels."), func() {
		a.settingNumber(c, a.text("卡片宽度", "Card width"), &s.CardWidth, 50, 480, 2)
		a.settingNumber(c, a.text("卡片高度", "Card height"), &s.CardHeight, 40, 360, 2)
		a.settingNumber(c, a.text("卡片字号", "Card font size"), &s.CardFontSize, 9, 18, 1)
		a.settingNumber(c, a.text("图标大小", "Icon size"), &s.CardIconScale, 16, min(s.CardWidth, s.CardHeight)*0.82, 2)
	})
	a.settingsSection(c, a.text("分类侧栏", "Library sidebar"), a.text("也可在主界面拖动分隔线调整。", "You can also drag the divider in the main window."), func() {
		a.settingNumber(c, a.text("分类栏宽度", "Sidebar width"), &s.SidebarWidth, 90, 320, 2)
	})
	a.settingsSection(c, a.text("卡片预览", "Card preview"), a.text("预览不会启动应用。", "This preview does not launch an application."), func() {
		ui.Scroll(c).Height(float32(s.CardHeight) + 24).Children(func() {
			ui.Column(c).Size(float32(s.CardWidth), float32(s.CardHeight)).Center().Gap(4).Padding(6).Radius(10).Background(c.Theme().Background).Border(1, c.Theme().Border).Children(func() {
				size := float32(s.CardIconScale * 0.72)
				ui.Box(c).Size(size, size).Radius(size * 0.25).Background(c.Theme().Accent.Alpha(0.12)).Center().Children(func() {
					ui.Text(c, "A").FontSize(size * 0.55).Bold().TextColor(c.Theme().Accent)
				})
				ui.Text(c, a.text("我的应用", "My application")).FontSize(float32(s.CardFontSize)).SingleLine().MaxWidth(float32(s.CardWidth - 12))
			})
		})
	})
}

func (a *App) behaviorSettings(c *ui.Context) {
	s := &a.state.Settings
	a.settingsSection(c, a.text("窗口", "Window"), "", func() {
		if a.settingToggle(c, a.text("窗口置顶", "Always on top"), a.text("让启动器显示在其他窗口上方。", "Keep the launcher above other windows."), &s.AlwaysOnTop) && a.window != nil {
			a.window.SetAlwaysOnTop(s.AlwaysOnTop)
		}
		a.settingToggle(c, a.text("双击卡片区域空白处隐藏", "Hide on double click in empty grid space"), a.text("有托盘或全局快捷键时可隐藏窗口。", "Hiding requires a tray or a global shortcut."), &s.DblClickBlankToHide)
		if a.settingToggle(c, a.text("记住窗口位置和大小", "Remember window position and size"), a.text("下次启动恢复窗口的位置和尺寸。", "Restore the window bounds on the next launch."), &s.RememberWindowBounds) {
			a.captureBounds()
		}
	})
	a.settingsSection(c, a.text("启动与路径", "Launch & paths"), "", func() {
		a.settingToggle(c, a.text("启动时隐藏窗口", "Hide window when the application starts"), a.text("程序启动后在后台待命。", "Start the launcher in the background."), &s.HideOnStartup)
		a.settingToggle(c, a.text("新增文件使用相对路径", "Use relative paths for newly added files"), a.text("以启动器程序所在目录为基准；只影响新增文件。", "Relative to the executable directory; applies to new files only."), &s.UseRelativePath)
	})
}

func (a *App) hotkeySettings(c *ui.Context) {
	a.settingsSection(c, a.text("全局快捷键", "Global shortcut"), a.text("在其他应用中也能显示或隐藏启动器。", "Show or hide the launcher from any application."), func() {
		ui.TextInput(c, &a.hotkey).Label(a.text("全局快捷键", "Global shortcut")).Placeholder("Ctrl+Alt+Space").Height(38)
		ui.Text(c, a.text("输入组合键；留空后应用可注销快捷键。", "Enter a key combination; apply an empty value to unregister it.")).FontSize(11).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(12).Children(func() {
			if ui.PrimaryButton(c, a.text("应用快捷键", "Apply shortcut")).Clicked() {
				if err := a.applyHotkey(a.hotkey); err != nil {
					a.report(err)
				} else {
					a.state.Settings.ToggleHotkey = a.registeredHotkey
					a.changed()
					a.message = a.text("快捷键已更新", "Shortcut updated")
				}
			}
			value := a.registeredHotkey
			if value == "" {
				value = a.text("未设置", "Not set")
			}
			ui.Text(c, a.text("当前：", "Current: ")+value).FontSize(11).TextColor(c.Theme().TextMuted)
		})
	})
	a.settingsSection(c, a.text("界面操作", "In-app shortcuts"), "", func() {
		for _, row := range []struct{ label, key string }{
			{a.text("搜索所有分类", "Search all groups"), "Ctrl / Cmd + F"},
			{a.text("清空搜索框", "Clear the search field"), "Esc"},
			{a.text("启动选中条目", "Launch selected shortcut"), "Enter / Space"},
		} {
			ui.Row(c).Gap(12).Children(func() {
				ui.Text(c, row.label).Grow(1)
				ui.Text(c, row.key).TextColor(c.Theme().TextMuted)
			})
		}
	})
}

func (a *App) settingsSection(c *ui.Context, title, description string, content func()) {
	ui.Column(c).Padding(18).Gap(14).Radius(12).Background(c.Theme().Surface).Border(1, c.Theme().Border.Alpha(0.7)).Children(func() {
		ui.Column(c).Gap(5).Children(func() {
			ui.Text(c, title).FontSize(15).Bold()
			if description != "" {
				ui.Text(c, description).FontSize(11).TextColor(c.Theme().TextMuted)
			}
		})
		content()
	})
}

func (a *App) settingRow(c *ui.Context, label, hint string, control func()) {
	ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(4).Children(func() {
			ui.Text(c, label)
			if hint != "" {
				ui.Text(c, hint).FontSize(11).TextColor(c.Theme().TextMuted)
			}
		})
		control()
	})
}

func (a *App) settingToggle(c *ui.Context, label, hint string, value *bool) bool {
	row := ui.SwitchBase(c, value).Label(label).Gap(16).AlignItems(ui.Center).Radius(6).Padding(4, 0)
	row.Children(func() {
		ui.Column(c).Grow(1).Gap(4).Children(func() {
			ui.Text(c, label)
			ui.Text(c, hint).FontSize(11).TextColor(c.Theme().TextMuted)
		})
		color, align := c.Theme().Border, ui.Start
		if *value {
			color, align = c.Theme().Accent, ui.End
		}
		ui.Row(c).Size(36, 20).Radius(10).Padding(2).Background(color).Justify(align).Children(func() {
			ui.Box(c).Size(16, 16).Radius(8).Background(ui.Hex("#ffffff"))
		})
	})
	if row.Changed() {
		a.changed()
		return true
	}
	return false
}

func (a *App) settingNumber(c *ui.Context, label string, value *float64, min, max, step float64) {
	a.settingRow(c, label, "", func() {
		if ui.NumberInput(c, value, min, max, step).Label(label).Width(160).Changed() {
			a.changed()
		}
	})
}

func (a *App) statusView(c *ui.Context) {
	ui.Row(c).Padding(8, 20).Gap(12).Children(func() {
		msg := a.message
		if msg == "" {
			msg = fmt.Sprintf(a.text("%d 个分类 · %d 个条目", "%d groups · %d items"), len(a.state.Groups), len(a.results))
		}
		ui.Text(c, msg).FontSize(11).TextColor(c.Theme().TextMuted).Grow(1).SingleLine().Tooltip(msg)
		if a.message != "" && ui.Button(c, a.text("清除提示", "Dismiss")).Clicked() {
			a.message = ""
		}
		ui.Text(c, a.themeName()).FontSize(11).TextColor(c.Theme().TextMuted)
		if a.registeredHotkey != "" {
			ui.Text(c, a.registeredHotkey).FontSize(11).TextColor(c.Theme().TextMuted)
		}
	})
}
