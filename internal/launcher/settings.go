package launcher

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/nsgo-dev/nsgo/core"
	nsw "github.com/nsgo-dev/nsgo/ui"

	"my-quickstart-go/internal/buildinfo"
	w "my-quickstart-go/internal/nativeui"
	"my-quickstart-go/internal/platform"
)

func (a *App) nativeAppearance() {
	cx := a.native.surface
	s := a.state.Settings
	p := palette(s.Theme)
	theme := core.DefaultTheme()
	color := func(c ui.Color) core.Color {
		return core.RGBA(uint32(c.R)<<16|uint32(c.G)<<8|uint32(c.B), float32(c.A)/255)
	}
	pal := &theme.Light
	mode := core.AppearanceLight
	if p.Dark {
		pal = &theme.Dark
		mode = core.AppearanceDark
	}
	pal.Background, pal.Surface, pal.Border = color(p.Background), color(p.Surface), color(p.Border)
	pal.Text, pal.TextMuted = color(p.Text), color(p.TextMuted)
	pal.Accent, pal.AccentText, pal.AccentHover, pal.AccentPressed = color(p.Accent), color(p.AccentText), color(p.AccentHover), color(p.AccentPressed)
	pal.SurfaceHover, pal.SurfacePressed, pal.Focus, pal.Selection = color(p.SurfaceHover), color(p.SurfacePressed), color(p.Focus), color(p.Selection)
	theme.Sans = fontName(s.FontFamily)
	cx.SetTheme(theme)
	cx.SetAppearance(mode)
	cx.SetLocale(s.Language)
	level := core.WindowLevelNormal
	if s.AlwaysOnTop {
		level = core.WindowLevelFloating
	}
	cx.SetWindowLevel(level)
}

func (a *App) nativeSettings(v w.Page) core.Node {
	t := a.text
	s := &a.state.Settings
	labels := []string{t("外观", "Appearance"), t("布局", "Layout"), t("行为", "Behavior"), t("快捷键", "Hotkey"), t("关于", "About"), t("资源导入", "Resource import")}
	back := v.Button(t("返回", "Back"), func() { a.settingsOpen = false })
	var bar []core.Node
	for i, label := range labels {
		variant := nsw.ButtonGhost
		if i == a.settingsTab {
			variant = nsw.ButtonPrimary
		}
		bar = append(bar, nsw.Button{Text: label, Variant: variant, OnPress: v.Action(func() { a.settingsTab = i })})
	}
	var items []core.Node
	toggle := func(label string, val *bool, invert bool) {
		checked := core.NewSignal(v.Scope, *val != invert)
		items = append(items, nsw.Checkbox{Text: label, Checked: checked.Get, OnChange: func(on bool) {
			checked.Set(on)
			*val = on != invert
			if val == &s.AlwaysOnTop {
				level := core.WindowLevelNormal
				if *val {
					level = core.WindowLevelFloating
				}
				v.Scope.SetWindowLevel(level)
			}
			a.queueSave()
			a.syncNative()
		}})
	}
	number := func(label string, val *float64, low, high, step float64) {
		items = append(items, w.Spread(w.Text(label), nsw.NumberInput{Label: label, Value: func() float64 { return *val }, Min: low, Max: high, Step: step, Style: core.Style{Width: core.Pt(130)}, OnChange: func(x float64) { *val = x; a.changed() }}))
	}
	apply := func() { a.nativeAppearance(); a.changed() }
	switch a.settingsTab {
	case 0:
		var names []string
		selected := ""
		for _, th := range themes {
			name := t(th.zh, th.en)
			names = append(names, name)
			if th.id == s.Theme {
				selected = name
			}
		}
		items = append(items, w.Row(w.Text(t("配色主题", "Color theme")), w.Select(t("主题", "Theme"), selected, names, func(name string) {
			for i, n := range names {
				if n == name {
					s.Theme = themes[i].id
					apply()
					return
				}
			}
		})),
			w.Row(w.Text(t("语言", "Language")), w.Select(t("语言", "Language"), s.Language, []string{"zh-CN", "en"}, func(x string) { s.Language = x; apply() })))
		choices := []string{"maye", "system", "segoe", "inter", "mono"}
		if a.fonts == nil {
			a.fonts = platform.Fonts()
		}
		choices = append(choices, a.fonts...)
		items = append(items, w.Row(w.Text(t("选择字体", "Choose font")), w.Select(t("选择字体", "Choose font"), s.FontFamily, choices, func(f string) { s.FontFamily = f; apply() })))
		font := s.FontFamily
		items = append(items, w.Field(t("字体", "Font"), &font, false), v.Button(t("应用字体", "Apply font"), func() { s.FontFamily = font; apply() }))
		number(t("界面字号", "UI font size"), &s.FontSize, 10, 22, 1)
	case 1:
		items = append(items, v.Muted(t("功能标签", "Feature tabs")))
		toggle(t("显示 MCP / Skills", "Show MCP / Skills"), &s.HideLibraryTab, true)
		toggle(t("显示定时任务", "Show scheduled tasks"), &s.HideScheduleTab, true)
		navigation := w.Column(items...)
		navigation.Style.FlexGrow = 0
		items = nil
		number(t("卡片宽度", "Card width"), &s.CardWidth, 50, 480, 2)
		number(t("卡片高度", "Card height"), &s.CardHeight, 40, 360, 2)
		number(t("卡片字号", "Card font size"), &s.CardFontSize, 9, 18, 1)
		number(t("图标大小", "Icon size"), &s.CardIconScale, 16, min(s.CardWidth, s.CardHeight)*0.82, 2)
		number(t("分类栏宽度", "Sidebar width"), &s.SidebarWidth, 90, 320, 2)
		dimensions := w.Column(items...)
		dimensions.Style.FlexBasis = core.Pt(0)
		dimensions.Style.FlexShrink = 1
		items = []core.Node{navigation}
		preview := v.Panel(v.Muted(t("卡片预览", "Card preview")), core.View{Style: core.Style{
			Width: core.Pt(core.Points(s.CardWidth)), Height: core.Pt(core.Points(s.CardHeight)), Background: v.Scope.Palette().Surface,
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, BorderWidth: 1, BorderColor: v.Scope.Palette().Border, CornerRadius: 8,
		}, Children: []core.Node{core.Text{Content: "A", Style: core.Style{FontSize: core.Points(s.CardIconScale * 0.55), Color: v.Scope.Palette().Accent}}, core.Text{Content: t("我的应用", "My application"), Style: core.Style{FontSize: core.Points(s.CardFontSize)}}}})
		preview.Style.Background = v.Scope.Palette().Background
		if v.Scope.Env().Window().Width >= core.Points(730+s.CardWidth) {
			items = append(items, core.View{Style: core.Style{Direction: core.Row, Gap: 24, AlignItems: core.AlignStart, Margin: core.Edges{Top: 10}}, Children: []core.Node{dimensions, preview}})
		} else {
			dimensions.Style.FlexBasis = core.Length{}
			items = append(items, dimensions, preview)
		}
	case 2:
		toggle(t("窗口置顶", "Always on top"), &s.AlwaysOnTop, false)
		toggle(t("双击空白处隐藏", "Double-click empty space to hide"), &s.DblClickBlankToHide, false)
		number(t("隐藏后返回应用启动（秒）", "Return to launcher after hiding (seconds)"), &s.ReturnToLauncherSeconds, 1, 3600, 1)
		toggle(t("记住窗口位置和大小", "Remember window position and size"), &s.RememberWindowBounds, false)
		toggle(t("启动时隐藏窗口", "Hide window on startup"), &s.HideOnStartup, false)
		toggle(t("新增文件使用相对路径", "Use relative paths for new files"), &s.UseRelativePath, false)
	case 3:
		items = append(items, w.Field(t("全局快捷键", "Global shortcut"), &a.hotkey, false), v.Button(t("应用快捷键", "Apply shortcut"), func() { a.native.hotkey.Set(strings.TrimSpace(a.hotkey)) }),
			w.Text(t("当前：", "Current: ")+a.registeredHotkey), w.Text(t("搜索  Ctrl / Cmd + F     收起  Esc     启动  Enter / Space", "Search  Ctrl / Cmd + F     Close search  Esc     Launch  Enter / Space")))
	case 4:
		items = append(items, w.Text("My Quickstart Go  "+buildinfo.Version), w.Text(runtime.Version()+"  "+runtime.GOOS+" / "+runtime.GOARCH))
		for _, d := range aboutDependencies {
			items = append(items, w.Text(d.Path+"  "+d.Version))
		}
	}
	heading := w.Spread(w.Heading(t("设置", "Settings")), back)
	content := v.Panel(w.Heading(labels[a.settingsTab]))
	content.Style.FlexGrow = 1
	content.Style.FlexShrink = 1
	content.Style.FlexBasis = core.Pt(0)
	if a.settingsTab == 5 && a.library != nil {
		content.Children = append(content.Children, a.library.NativeImport(v))
	} else {
		content.Children = append(content.Children, w.Form(items...))
	}
	if v.Scope.Env().Window().Width < 700 {
		return w.Column(heading, w.Row(bar...), content)
	}
	rail := v.Panel(bar...)
	rail.Style.Width = core.Pt(156)
	layout := core.View{Style: core.Style{Direction: core.Row, Gap: 12, FlexGrow: 1}, Children: []core.Node{rail, content}}
	return w.Column(heading, layout)
}
